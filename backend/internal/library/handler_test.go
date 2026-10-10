package library

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"codeberg.org/pfrappier/openboxd/backend/internal/database"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewStore(db)
}

type watchedPage struct {
	Total int
	Films []WatchedFilm
}

func getWatched(t *testing.T, store *Store, query string) (*httptest.ResponseRecorder, watchedPage) {
	t.Helper()
	rec := httptest.NewRecorder()
	NewHandler(store).Watched(rec, httptest.NewRequest(http.MethodGet, "/api/watched"+query, nil))

	var page watchedPage
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
	}
	return rec, page
}

func TestWatchedEmpty(t *testing.T) {
	rec, _ := getWatched(t, newStore(t), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if want := `{"total":0,"films":[]}` + "\n"; rec.Body.String() != want {
		t.Errorf("body = %s, want %s", rec.Body, want)
	}
}

func TestWatchedPagination(t *testing.T) {
	store := newStore(t)
	var films []WatchedFilm
	for i := range 120 {
		films = append(films, WatchedFilm{
			Name:          fmt.Sprintf("Film %03d", i),
			LetterboxdURI: fmt.Sprintf("https://boxd.it/%d", i),
			// Day 1 to 28, then again: several films share a date.
			WatchedOn: fmt.Sprintf("2024-01-%02d", i%28+1),
		})
	}
	if err := store.AddWatched(t.Context(), films); err != nil {
		t.Fatal(err)
	}

	_, first := getWatched(t, store, "")
	if first.Total != 120 || len(first.Films) != defaultPageSize {
		t.Fatalf("default page: total = %d, %d films", first.Total, len(first.Films))
	}
	if got := first.Films[0]; got.WatchedOn != "2024-01-28" || got.Name != "Film 027" {
		t.Errorf("first film = %+v, want the latest date then the name order", got)
	}

	seen := map[int64]bool{}
	for offset := 0; offset < 120; offset += 50 {
		_, page := getWatched(t, store, fmt.Sprintf("?limit=50&offset=%d", offset))
		for _, f := range page.Films {
			if seen[f.ID] {
				t.Errorf("film %d returned by two pages", f.ID)
			}
			seen[f.ID] = true
		}
	}
	if len(seen) != 120 {
		t.Errorf("pages returned %d distinct films, want 120", len(seen))
	}

	if _, page := getWatched(t, store, "?limit=100000"); len(page.Films) != 120 {
		t.Errorf("oversized limit returned %d films, want all 120 (under the %d cap)", len(page.Films), maxPageSize)
	}
	if _, page := getWatched(t, store, "?offset=500"); page.Total != 120 || len(page.Films) != 0 {
		t.Errorf("offset past the end: total = %d, %d films", page.Total, len(page.Films))
	}
}

func TestWatchedBadQuery(t *testing.T) {
	store := newStore(t)
	for _, query := range []string{"?limit=0", "?limit=-1", "?limit=abc", "?offset=-1", "?offset=x"} {
		if rec, _ := getWatched(t, store, query); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", query, rec.Code)
		}
	}
}
