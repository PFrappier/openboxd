package imports

import (
	"encoding/csv"
	"io"
	"io/fs"
	"slices"
	"strconv"

	"codeberg.org/pfrappier/openboxd/backend/internal/library"
)

// readList parses a file of lists/. After a line telling the version of the
// format, it holds two tables: the list itself, in a single row, then its
// films, in the order of the list. ok is false for a file that lacks the URI
// of the list: it is what identifies it.
func readList(export fs.FS, name string) (list library.ImportedList, ok bool, err error) {
	err = readCSV(export, name, func(r *csv.Reader) error {
		// The file ending early leaves the list with what was read of it.
		if err := parseList(r, &list); err != io.EOF {
			return err
		}
		return nil
	})
	return list, list.LetterboxdURI != "", err
}

// parseList fills list from its file, until the error that ends it: io.EOF
// for a file read through.
func parseList(r *csv.Reader, list *library.ImportedList) error {
	column, err := listTable(r)
	if err != nil {
		return err
	}
	date, title, uri, description := column("Date"), column("Name"), column("URL"), column("Description")
	row, err := r.Read()
	if err != nil {
		return err
	}
	list.CreatedOn, list.Name, list.LetterboxdURI, list.Description = date(row), title(row), uri(row), description(row)

	column, err = listTable(r)
	if err != nil {
		return err
	}
	title, year, uri := column("Name"), column("Year"), column("URL")
	for {
		row, err := r.Read()
		if err != nil {
			return err
		}
		// As in the other files, a film without a URI can't be told.
		if uri(row) == "" {
			continue
		}
		film := library.ListFilm{Name: title(row), LetterboxdURI: uri(row)}
		if y, err := strconv.Atoi(year(row)); err == nil {
			film.Year = &y
		}
		list.Films = append(list.Films, film)
	}
}

// listTable skips to the next table of a list file, which both start with a
// header that has a URL column, and returns what reads its columns.
func listTable(r *csv.Reader) (func(title string) func(row []string) string, error) {
	for {
		row, err := r.Read()
		if err != nil {
			return nil, err
		}
		if slices.Contains(row, "URL") {
			return columnsOf(row), nil
		}
	}
}
