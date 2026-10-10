package imports

import (
	"archive/zip"
	"encoding/csv"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Summary describes what a stored export contains.
type Summary struct {
	ID string `json:"id"`
	// Username comes from profile.csv and is null when the file is missing.
	Username *string `json:"username"`
	Counts   Counts  `json:"counts"`
}

// Counts holds the number of entries per kind of data. Lists is the number of
// list files, the others are row counts.
type Counts struct {
	Watched   int `json:"watched"`
	Ratings   int `json:"ratings"`
	Diary     int `json:"diary"`
	Reviews   int `json:"reviews"`
	Watchlist int `json:"watchlist"`
	Likes     int `json:"likes"`
	Lists     int `json:"lists"`
}

// Summary handles GET /api/imports/{id}. The export is read again on every
// call: it is a few MB of CSV at most, and nothing is persisted yet.
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isImportID(id) {
		writeError(w, http.StatusNotFound, "import not found")
		return
	}

	export, closeExport, err := openExport(filepath.Join(h.dir, id))
	if errors.Is(err, fs.ErrNotExist) {
		writeError(w, http.StatusNotFound, "import not found")
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "open export", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, "could not read the export")
		return
	}
	defer closeExport()

	summary, err := summarize(export)
	if err != nil {
		slog.ErrorContext(r.Context(), "summarize export", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, "could not read the export")
		return
	}
	summary.ID = id
	writeJSON(w, http.StatusOK, summary)
}

// isImportID reports whether id looks like one made by Upload, which keeps
// anything else, path separators included, away from the file system.
func isImportID(id string) bool {
	if id == "" {
		return false
	}
	for _, c := range id {
		if (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// openExport returns the files of the import stored in dir, whether it was
// uploaded as a ZIP archive or as an unzipped folder.
func openExport(dir string) (fs.FS, func() error, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, nil, err
	}
	zr, err := zip.OpenReader(filepath.Join(dir, zipFileName))
	if errors.Is(err, fs.ErrNotExist) {
		return os.DirFS(dir), func() error { return nil }, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return zr, zr.Close, nil
}

func summarize(export fs.FS) (Summary, error) {
	var s Summary
	err := fs.WalkDir(export, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(path.Ext(name), ".csv") {
			return nil
		}

		segments := strings.Split(name, "/")
		// Entries the user deleted on Letterboxd are exported too.
		if slices.Contains(segments, "deleted") || slices.Contains(segments, "orphaned") {
			return nil
		}
		// Matching on the parent folder rather than the full path also accepts
		// archives that wrap the export in a root folder.
		parent := path.Base(path.Dir(name))
		base := path.Base(name)

		switch {
		case parent == "lists":
			s.Counts.Lists++
			return nil
		case parent == "likes":
			if base == "films.csv" {
				return countRows(export, name, &s.Counts.Likes)
			}
			return nil
		}

		switch base {
		case "profile.csv":
			username, err := readUsername(export, name)
			if err != nil {
				return err
			}
			s.Username = username
		case "watched.csv":
			return countRows(export, name, &s.Counts.Watched)
		case "ratings.csv":
			return countRows(export, name, &s.Counts.Ratings)
		case "diary.csv":
			return countRows(export, name, &s.Counts.Diary)
		case "reviews.csv":
			return countRows(export, name, &s.Counts.Reviews)
		case "watchlist.csv":
			return countRows(export, name, &s.Counts.Watchlist)
		}
		return nil
	})
	return s, err
}

// countRows adds the number of records after the header row to n.
func countRows(export fs.FS, name string, n *int) error {
	return readCSV(export, name, func(r *csv.Reader) error {
		if _, err := r.Read(); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		for {
			_, err := r.Read()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			*n++
		}
	})
}

// readUsername returns the Username column of profile.csv, or nil without one.
func readUsername(export fs.FS, name string) (*string, error) {
	var username *string
	err := readCSV(export, name, func(r *csv.Reader) error {
		header, err := r.Read()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		col := slices.IndexFunc(header, func(h string) bool {
			return strings.TrimPrefix(h, "\ufeff") == "Username"
		})
		row, err := r.Read()
		if err != nil && err != io.EOF {
			return err
		}
		if col >= 0 && col < len(row) && row[col] != "" {
			username = &row[col]
		}
		return nil
	})
	return username, err
}

func readCSV(export fs.FS, name string, read func(*csv.Reader) error) error {
	f, err := export.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	r := csv.NewReader(f)
	// Reviews are free text: tolerate stray quotes and uneven rows instead of
	// failing the whole summary.
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.ReuseRecord = true
	return read(r)
}
