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
	err = walkExport(export, func(name string, kind fileKind) error {
		switch kind {
		case watchedFile, ratingsFile, likedFilmsFile:
		default:
			return nil
		}
		rows, err := readFilmRows(export, name)
		if err != nil {
			return err
		}
		for _, row := range rows {
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
	return h.store.Import(ctx, imp)
}

// filmRow is a row of one of the files that list films: watched.csv,
// ratings.csv and likes/films.csv share the columns Date, Name, Year and
// Letterboxd URI.
type filmRow struct {
	// date is the day of the action the file is about, e.g. rating the film.
	date string
	name string
	// year is nil for films without a release year on Letterboxd.
	year *int
	uri  string
	// rating is the Rating column, which only ratings.csv has.
	rating string
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
		column := func(title string) func(row []string) string {
			i := slices.Index(header, title)
			return func(row []string) string {
				if i < 0 || i >= len(row) {
					return ""
				}
				return strings.TrimSpace(row[i])
			}
		}
		date, title, year, uri := column("Date"), column("Name"), column("Year"), column("Letterboxd URI")
		rating := column("Rating")

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
			film := filmRow{date: date(row), name: title(row), uri: uri(row), rating: rating(row)}
			if y, err := strconv.Atoi(year(row)); err == nil {
				film.year = &y
			}
			films = append(films, film)
		}
	})
	return films, err
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
