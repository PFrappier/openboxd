package imports

import (
	"context"
	"encoding/csv"
	"io"
	"io/fs"
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

	var watched []library.WatchedFilm
	err = walkExport(export, func(name string, kind fileKind) error {
		if kind != watchedFile {
			return nil
		}
		films, err := readWatched(export, name)
		watched = append(watched, films...)
		return err
	})
	if err != nil {
		return err
	}
	return h.store.AddWatched(ctx, watched)
}

// readWatched parses watched.csv, whose columns are Date, Name, Year and
// Letterboxd URI. Rows without a URI are skipped: it is what identifies a film.
func readWatched(export fs.FS, name string) ([]library.WatchedFilm, error) {
	var films []library.WatchedFilm
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
			film := library.WatchedFilm{
				Name:          title(row),
				LetterboxdURI: uri(row),
				WatchedOn:     date(row),
			}
			if y, err := strconv.Atoi(year(row)); err == nil {
				film.Year = &y
			}
			films = append(films, film)
		}
	})
	return films, err
}
