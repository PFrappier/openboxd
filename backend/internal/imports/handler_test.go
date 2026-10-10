package imports

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/pfrappier/openboxd/backend/internal/database"
	"codeberg.org/pfrappier/openboxd/backend/internal/library"
)

type upload struct {
	name    string
	content []byte
}

func newRequest(t *testing.T, files ...upload) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, f := range files {
		// CreateFormFile would escape the name but keeps the slashes we need.
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="export"; filename="`+f.name+`"`)
		w, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(f.content)
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/imports", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func zipBytes(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range names {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte("Date,Name\n"))
	}
	zw.Close()
	return buf.Bytes()
}

// newHandler returns a handler storing exports under dir, backed by a library
// in its own temporary database.
func newHandler(t *testing.T, dir string) (*Handler, *library.Store) {
	t.Helper()
	db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store := library.NewStore(db)
	return NewHandler(dir, store, func() {}), store
}

func serve(t *testing.T, req *http.Request) (*httptest.ResponseRecorder, string) {
	t.Helper()
	dir := t.TempDir()
	h, _ := newHandler(t, dir)
	rec := httptest.NewRecorder()
	h.Upload(rec, req)
	return rec, dir
}

func importID(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp struct{ ID string }
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil || resp.ID == "" {
		t.Fatalf("missing import id in %q", rec.Body.String())
	}
	return resp.ID
}

func TestUploadZip(t *testing.T) {
	req := newRequest(t, upload{"letterboxd.zip", zipBytes(t, "watched.csv", "lists/top.csv")})
	rec, dir := serve(t, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	id := importID(t, rec)
	if _, err := os.Stat(filepath.Join(dir, id, zipFileName)); err != nil {
		t.Fatal(err)
	}
}

func TestUploadFolder(t *testing.T) {
	req := newRequest(t,
		upload{"letterboxd-user/watched.csv", []byte("Date,Name\n")},
		upload{"letterboxd-user/lists/top.csv", []byte("Date,Name\n")},
		upload{"letterboxd-user/.DS_Store", []byte{0}},
	)
	rec, dir := serve(t, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	id := importID(t, rec)
	for _, rel := range []string{"watched.csv", "lists/top.csv"} {
		if _, err := os.Stat(filepath.Join(dir, id, rel)); err != nil {
			t.Error(err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, id, ".DS_Store")); !os.IsNotExist(err) {
		t.Error(".DS_Store should not be stored")
	}
}

func TestUploadRejected(t *testing.T) {
	tests := map[string][]upload{
		"empty":           nil,
		"not a zip":       {{"export.zip", []byte("nope")}},
		"zip without csv": {{"export.zip", zipBytes(t, "readme.txt")}},
		"unknown format":  {{"export.pdf", []byte("%PDF")}},
		"path traversal":  {{"root/../../evil.csv", []byte("x")}},
		"zip and folder":  {{"root/watched.csv", []byte("x")}, {"export.zip", zipBytes(t, "watched.csv")}},
		"duplicate file":  {{"root/watched.csv", []byte("x")}, {"root/watched.csv", []byte("x")}},
	}
	for name, files := range tests {
		t.Run(name, func(t *testing.T) {
			rec, dir := serve(t, newRequest(t, files...))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Errorf("rejected upload left %d entries on disk", len(entries))
			}
		})
	}
}

func TestUploadNotMultipart(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/imports", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec, _ := serve(t, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}
