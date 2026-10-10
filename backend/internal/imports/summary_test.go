package imports

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const filmHeader = "Date,Name,Year,Letterboxd URI\n"

// exportFiles is a small but complete export: every count differs so that a
// mix-up between two files shows in the summary.
var exportFiles = map[string]string{
	"profile.csv":       "Date Joined,Username,Given Name\n2020-01-01,cinephile,Ada\n",
	"watched.csv":       filmHeader + "2024-01-01,Alien,1979,a\n2024-01-02,Heat,1995,b\n2024-01-03,Ran,1985,c\n",
	"ratings.csv":       filmHeader + "2024-01-01,Alien,1979,a\n2024-01-02,Heat,1995,b\n",
	"diary.csv":         filmHeader + "2024-01-01,Alien,1979,a\n",
	"reviews.csv":       "Date,Name,Review\n2024-01-01,Alien,\"In space,\nno one can hear you \"\"scream\"\".\"\n",
	"watchlist.csv":     filmHeader + "2024-01-01,Alien,1979,a\n2024-01-02,Heat,1995,b\n2024-01-03,Ran,1985,c\n2024-01-04,Tampopo,1985,d\n",
	"comments.csv":      "Date,Content\n2024-01-01,hello\n",
	"likes/films.csv":   filmHeader + "2024-01-01,Alien,1979,a\n2024-01-02,Heat,1995,b\n2024-01-03,Ran,1985,c\n2024-01-04,Tampopo,1985,d\n2024-01-05,Persona,1966,e\n",
	"likes/reviews.csv": "Date,Name\n2024-01-01,Alien\n",
	"lists/top.csv":     "Letterboxd list export v7\nDate,Name\n2024-01-01,Top\n\nPosition,Name\n1,Alien\n",
	"lists/sci-fi.csv":  "Letterboxd list export v7\nDate,Name\n2024-01-01,Sci-fi\n\nPosition,Name\n1,Alien\n",
	"deleted/diary.csv": filmHeader + "2024-01-01,Alien,1979,a\n",
}

var wantCounts = Counts{Watched: 3, Ratings: 2, Diary: 1, Reviews: 1, Watchlist: 4, Likes: 5, Lists: 2}

func zipExport(t *testing.T, prefix string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(prefix + name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
	zw.Close()
	return buf.Bytes()
}

// uploadAndSummarize stores files through Upload, then asks for their summary.
func uploadAndSummarize(t *testing.T, files ...upload) Summary {
	t.Helper()
	h, _ := newHandler(t, t.TempDir())

	rec := httptest.NewRecorder()
	h.Upload(rec, newRequest(t, files...))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, body = %s", rec.Code, rec.Body)
	}
	id := importID(t, rec)

	rec = getSummary(h, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("summary status = %d, body = %s", rec.Code, rec.Body)
	}
	var s Summary
	if err := json.NewDecoder(rec.Body).Decode(&s); err != nil {
		t.Fatal(err)
	}
	if s.ID != id {
		t.Errorf("id = %q, want %q", s.ID, id)
	}
	return s
}

func getSummary(h *Handler, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/imports/"+id, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	h.Summary(rec, req)
	return rec
}

func checkSummary(t *testing.T, s Summary) {
	t.Helper()
	if s.Counts != wantCounts {
		t.Errorf("counts = %+v, want %+v", s.Counts, wantCounts)
	}
	if s.Username == nil || *s.Username != "cinephile" {
		t.Errorf("username = %v, want cinephile", s.Username)
	}
}

func TestSummaryZip(t *testing.T) {
	checkSummary(t, uploadAndSummarize(t, upload{"export.zip", zipExport(t, "", exportFiles)}))
}

func TestSummaryZipWithRootFolder(t *testing.T) {
	archive := zipExport(t, "letterboxd-user/", exportFiles)
	checkSummary(t, uploadAndSummarize(t, upload{"export.zip", archive}))
}

func TestSummaryFolder(t *testing.T) {
	var files []upload
	for name, content := range exportFiles {
		files = append(files, upload{"letterboxd-user/" + name, []byte(content)})
	}
	checkSummary(t, uploadAndSummarize(t, files...))
}

func TestSummaryPartialExport(t *testing.T) {
	s := uploadAndSummarize(t, upload{"letterboxd-user/watched.csv", []byte(filmHeader)})

	if s.Counts != (Counts{}) {
		t.Errorf("counts = %+v, want all zero", s.Counts)
	}
	if s.Username != nil {
		t.Errorf("username = %q, want nil", *s.Username)
	}
}

func TestSummaryJSONShape(t *testing.T) {
	h, _ := newHandler(t, t.TempDir())
	rec := httptest.NewRecorder()
	h.Upload(rec, newRequest(t, upload{"root/watched.csv", []byte(filmHeader)}))
	id := importID(t, rec)

	want := `{"id":"` + id + `","username":null,"counts":{"watched":0,"ratings":0,"diary":0,"reviews":0,"watchlist":0,"likes":0,"lists":0}}` + "\n"
	if got := getSummary(h, id).Body.String(); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestSummaryNotFound(t *testing.T) {
	h, _ := newHandler(t, t.TempDir())
	for _, id := range []string{"UNKNOWNIMPORTID", "", "..", "../imports", "a/b", "lower"} {
		if rec := getSummary(h, id); rec.Code != http.StatusNotFound {
			t.Errorf("id %q: status = %d, want 404", id, rec.Code)
		}
	}
}
