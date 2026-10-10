// Package imports receives Letterboxd data exports.
//
// An upload is validated and stored on disk under its own directory, named
// after the returned import ID, then imported into the library. Only watched
// films are imported for now; the stored export lets later versions import
// the rest without asking the user for it again.
package imports

import (
	"archive/zip"
	"crypto/rand"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"codeberg.org/pfrappier/openboxd/backend/internal/library"
	"codeberg.org/pfrappier/openboxd/backend/internal/respond"
)

const (
	// Letterboxd exports are CSV files weighing a few MB at most.
	maxUploadSize = 100 << 20
	formField     = "export"
	zipFileName   = "export.zip"
	// Replaces the server-wide read/write timeouts for this route: enough for
	// maxUploadSize at ~350 KB/s.
	uploadTimeout = 5 * time.Minute
)

var errInvalidExport = errors.New("expected a Letterboxd export: a .zip archive or the files of the unzipped folder")

type Handler struct {
	dir      string
	store    *library.Store
	imported func()
}

// NewHandler returns a handler that keeps uploaded exports under dir and
// imports their content into store. It calls imported after each import.
func NewHandler(dir string, store *library.Store, imported func()) *Handler {
	return &Handler{dir: dir, store: store, imported: imported}
}

// Upload handles a multipart request whose "export" field holds either the
// ZIP archive, or every file of the unzipped folder (Safari unzips it on
// download). In the folder case each part's filename must be its relative
// path, e.g. "letterboxd-user-2026-10-10/ratings.csv".
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	extendDeadlines(w, r)
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	mr, err := r.MultipartReader()
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "expected a multipart/form-data request")
		return
	}

	id := rand.Text()
	dest := filepath.Join(h.dir, id)
	if err := os.MkdirAll(dest, 0o750); err != nil {
		slog.ErrorContext(r.Context(), "create import dir", "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not store the export")
		return
	}

	if err := saveExport(mr, dest); err != nil {
		os.RemoveAll(dest)
		var maxBytesErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxBytesErr):
			respond.Error(w, http.StatusRequestEntityTooLarge, "export is too large")
		case errors.Is(err, errInvalidExport):
			respond.Error(w, http.StatusBadRequest, err.Error())
		default:
			slog.ErrorContext(r.Context(), "save export", "err", err)
			respond.Error(w, http.StatusInternalServerError, "could not store the export")
		}
		return
	}

	if err := h.importExport(r.Context(), dest); err != nil {
		os.RemoveAll(dest)
		slog.ErrorContext(r.Context(), "import export", "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not import the export")
		return
	}
	h.imported()

	respond.JSON(w, http.StatusCreated, map[string]string{"id": id})
}

func extendDeadlines(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	deadline := time.Now().Add(uploadTimeout)
	if err := rc.SetReadDeadline(deadline); err != nil {
		slog.WarnContext(r.Context(), "extend read deadline", "err", err)
	}
	if err := rc.SetWriteDeadline(deadline); err != nil {
		slog.WarnContext(r.Context(), "extend write deadline", "err", err)
	}
}

func saveExport(mr *multipart.Reader, dest string) error {
	var isZip bool
	var csvCount int

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if part.FormName() != formField {
			continue
		}

		name := rawFileName(part)
		switch strings.ToLower(path.Ext(name)) {
		case ".zip":
			if isZip || csvCount > 0 {
				return errInvalidExport
			}
			n, err := saveZip(part, dest)
			if err != nil {
				return err
			}
			isZip, csvCount = true, n
		case ".csv":
			if isZip {
				return errInvalidExport
			}
			if err := saveCSV(part, name, dest); err != nil {
				return err
			}
			csvCount++
		default:
			// Folder uploads can carry unrelated files such as .DS_Store.
		}
	}

	if csvCount == 0 {
		return errInvalidExport
	}
	return nil
}

// saveZip stores the archive and returns how many CSV files it contains.
func saveZip(part io.Reader, dest string) (int, error) {
	zipPath := filepath.Join(dest, zipFileName)
	if err := saveFile(part, zipPath); err != nil {
		return 0, err
	}
	return countZipCSVs(zipPath)
}

// saveCSV stores one file of an unzipped export under its relative path.
func saveCSV(part io.Reader, name, dest string) error {
	rel, ok := relativeExportPath(name)
	if !ok {
		return errInvalidExport
	}
	err := saveFile(part, filepath.Join(dest, rel))
	if errors.Is(err, fs.ErrExist) {
		return errInvalidExport
	}
	return err
}

// rawFileName returns the part's filename as sent by the client.
// multipart.Part.FileName strips directories, which folder uploads need.
func rawFileName(part *multipart.Part) string {
	_, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	if err != nil {
		return ""
	}
	return params["filename"]
}

// relativeExportPath drops the export's root folder from name and rejects
// anything that would escape the import directory.
func relativeExportPath(name string) (string, bool) {
	if _, rest, found := strings.Cut(name, "/"); found {
		name = rest
	}
	rel := filepath.FromSlash(name)
	return rel, filepath.IsLocal(rel)
}

func countZipCSVs(zipPath string) (int, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, errInvalidExport
	}
	defer zr.Close()

	n := 0
	for _, f := range zr.File {
		if strings.EqualFold(path.Ext(f.Name), ".csv") {
			n++
		}
	}
	return n, nil
}

func saveFile(src io.Reader, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, src); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
