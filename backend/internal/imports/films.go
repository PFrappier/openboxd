package imports

import (
	"context"
	"encoding/csv"
	"io"
	"io/fs"
	"math"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/pfrappier/openboxd/backend/internal/library"
)

// importExport adds the content of the export stored in dir to the library.
func (h *Handler) importExport(ctx context.Context, dir string) error {
	export, closeExport, err := openExport(dir)
	if err != nil {
		return err
	}
	defer closeExport()

	var imp library.Import
	// The diary is matched with films once the files that list them are read.
	var diary []filmRow
	films := filmIndex{}
	err = walkExport(export, func(name string, kind fileKind) error {
		switch kind {
		case watchedFile, ratingsFile, likedFilmsFile, diaryFile, watchlistFile:
		case listFile:
			list, ok, err := readList(export, name)
			if ok {
				imp.Lists = append(imp.Lists, list)
			}
			return err
		default:
			return nil
		}
		rows, err := readFilmRows(export, name)
		if err != nil {
			return err
		}
		switch kind {
		case diaryFile:
			diary = append(diary, rows...)
			return nil
		case watchlistFile:
			// Kept out of the index: the films of the diary are watched ones.
			for _, row := range rows {
				imp.Watchlist = append(imp.Watchlist, library.WatchlistFilm{
					Name: row.name, Year: row.year, LetterboxdURI: row.uri, AddedOn: row.date,
				})
			}
			return nil
		}
		for _, row := range rows {
			films.add(row)
			switch kind {
			case watchedFile:
				imp.Watched = append(imp.Watched, library.WatchedFilm{
					Name: row.name, Year: row.year, LetterboxdURI: row.uri, WatchedOn: row.date,
				})
			case ratingsFile:
				// A rating Openboxd can't represent is left out rather than
				// failing the whole import.
				halfStars, ok := parseRating(row.rating)
				if !ok {
					continue
				}
				imp.Ratings = append(imp.Ratings, library.RatedFilm{
					Name: row.name, Year: row.year, LetterboxdURI: row.uri,
					HalfStars: halfStars, RatedOn: row.date,
				})
			case likedFilmsFile:
				imp.Likes = append(imp.Likes, library.LikedFilm{
					Name: row.name, Year: row.year, LetterboxdURI: row.uri, LikedOn: row.date,
				})
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	for _, row := range diary {
		// Logging a film marks it as watched, so watched.csv lists it. An
		// entry whose film can't be told is left out.
		filmURI, ok := films.uri(row)
		if !ok {
			continue
		}
		entry := library.LoggedFilm{
			Name: row.name, Year: row.year, LetterboxdURI: filmURI, EntryURI: row.uri,
			WatchedOn: row.watchedDate, Rewatch: row.rewatch,
		}
		if entry.WatchedOn == "" {
			entry.WatchedOn = row.date
		}
		// Viewings can be logged without a rating.
		entry.HalfStars, _ = parseRating(row.rating)
		imp.Diary = append(imp.Diary, entry)
	}
	return h.store.Import(ctx, imp)
}

// filmRow is a row of one of the files that list films: watched.csv,
// ratings.csv, likes/films.csv, diary.csv and watchlist.csv share the columns
// Date, Name, Year and Letterboxd URI.
type filmRow struct {
	// date is the day of the action the file is about, e.g. rating the film.
	date string
	name string
	// year is nil for films without a release year on Letterboxd.
	year *int
	// uri is the one of the film, except in diary.csv: the one of the entry.
	uri string
	// rating is the Rating column, which ratings.csv and diary.csv have.
	rating string
	// rewatch and watchedDate are the Rewatch and Watched Date columns of
	// diary.csv, whose date is the day the viewing was logged.
	rewatch     bool
	watchedDate string
}

// filmIndex finds the URI of a film from its name and release year, the only
// things diary.csv tells about the film of an entry.
type filmIndex map[filmKey]string

type filmKey struct {
	name string
	// year is 0 for films without a release year.
	year int
}

func keyOf(row filmRow) filmKey {
	key := filmKey{name: row.name}
	if row.year != nil {
		key.year = *row.year
	}
	return key
}

// ambiguous stands for the URI of films that share their name and year.
const ambiguous = ""

func (idx filmIndex) add(row filmRow) {
	key := keyOf(row)
	if known, ok := idx[key]; ok && known != row.uri {
		idx[key] = ambiguous
		return
	}
	idx[key] = row.uri
}

// uri returns the URI of the film a diary row is about, unless no film or
// several ones have its name and year.
func (idx filmIndex) uri(row filmRow) (string, bool) {
	uri := idx[keyOf(row)]
	return uri, uri != ambiguous
}

// readFilmRows parses a file that lists films. Rows without a URI are
// skipped: it is what identifies a film.
func readFilmRows(export fs.FS, name string) ([]filmRow, error) {
	var films []filmRow
	err := readCSV(export, name, func(r *csv.Reader) error {
		header, err := r.Read()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
		column := columnsOf(header)
		date, title, year, uri := column("Date"), column("Name"), column("Year"), column("Letterboxd URI")
		rating, rewatch, watchedDate := column("Rating"), column("Rewatch"), column("Watched Date")

		for {
			row, err := r.Read()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			if uri(row) == "" {
				continue
			}
			film := filmRow{
				date: date(row), name: title(row), uri: uri(row), rating: rating(row),
				rewatch: rewatch(row) == "Yes", watchedDate: watchedDate(row),
			}
			if y, err := strconv.Atoi(year(row)); err == nil {
				film.year = &y
			}
			films = append(films, film)
		}
	})
	return films, err
}

// columnsOf returns what gives, from the title of a column of header, what
// reads that column in a row; a column the file lacks reads as empty. The
// header isn't kept: it can be overwritten once the columns are known.
func columnsOf(header []string) func(title string) func(row []string) string {
	return func(title string) func(row []string) string {
		i := slices.Index(header, title)
		return func(row []string) string {
			if i < 0 || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
	}
}

// parseRating converts a Letterboxd rating, from "0.5" to "5" stars, to half
// stars.
func parseRating(stars string) (int, bool) {
	n, err := strconv.ParseFloat(stars, 64)
	if err != nil {
		return 0, false
	}
	halfStars := n * 2
	if halfStars < 1 || halfStars > 10 || halfStars != math.Trunc(halfStars) {
		return 0, false
	}
	return int(halfStars), true
}
