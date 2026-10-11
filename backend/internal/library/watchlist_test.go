package library

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type watchlistPage struct {
	Total int
	Films []WatchlistFilm
}

func getWatchlist(t *testing.T, store *Store, query string) (*httptest.ResponseRecorder, watchlistPage) {
	t.Helper()
	rec := httptest.NewRecorder()
	NewHandler(store).Watchlist(rec, httptest.NewRequest(http.MethodGet, "/api/watchlist"+query, nil))

	var page watchlistPage
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
	}
	return rec, page
}

func watchlistNames(page watchlistPage) string {
	names := ""
	for _, f := range page.Films {
		names += f.Name
	}
	return names
}

func TestWatchlistEmpty(t *testing.T) {
	rec, _ := getWatchlist(t, newStore(t), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if want := `{"total":0,"films":[]}` + "\n"; rec.Body.String() != want {
		t.Errorf("body = %s, want %s", rec.Body, want)
	}
}

func TestWatchlist(t *testing.T) {
	const matrix, alien = "https://boxd.it/2a1m", "https://boxd.it/2awY"
	y1979 := 1979
	store := newStore(t)
	imp := Import{
		// Watched films aren't in the watchlist, and the other way around.
		Watched: []WatchedFilm{{Name: "Heat", LetterboxdURI: "https://boxd.it/2bfM", WatchedOn: "2024-01-01"}},
		Watchlist: []WatchlistFilm{
			{Name: "The Matrix", LetterboxdURI: matrix, AddedOn: "2024-01-02"},
			{Name: "Alien", Year: &y1979, LetterboxdURI: alien, AddedOn: "2024-03-05"},
		},
	}
	// Importing again doesn't duplicate the films.
	for range 2 {
		if err := store.Import(t.Context(), imp); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetMetadata(t.Context(), 2, Metadata{TMDBID: 603, PosterPath: "/matrix.jpg", Runtime: 136}); err != nil {
		t.Fatal(err)
	}

	rec, _ := getWatchlist(t, store, "")
	want := `{"total":2,"films":[` +
		`{"id":3,"name":"Alien","year":1979,"letterboxdUri":"https://boxd.it/2awY","addedOn":"2024-03-05",` +
		`"posterPath":null,"runtime":null},` +
		`{"id":2,"name":"The Matrix","year":null,"letterboxdUri":"https://boxd.it/2a1m","addedOn":"2024-01-02",` +
		`"posterPath":"/matrix.jpg","runtime":136}]}` + "\n"
	if rec.Body.String() != want {
		t.Errorf("body = %s\nwant %s", rec.Body, want)
	}
	if _, page := getWatched(t, store, ""); page.Total != 1 {
		t.Errorf("films of the watchlist count as watched: total = %d, want 1", page.Total)
	}

	// The film tells when it joined the watchlist.
	film, err := store.Film(t.Context(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if film.WatchlistedOn == nil || *film.WatchlistedOn != "2024-01-02" || film.WatchedOn != nil {
		t.Errorf("film of the watchlist = %+v", film)
	}
	if film, err = store.Film(t.Context(), 1); err != nil || film.WatchlistedOn != nil {
		t.Errorf("watched film: watchlisted on %v, err = %v, want neither", film.WatchlistedOn, err)
	}

	// A film added again on Letterboxd is updated.
	imp.Watchlist = []WatchlistFilm{{Name: "The Matrix", LetterboxdURI: matrix, AddedOn: "2024-06-01"}}
	if err := store.Import(t.Context(), imp); err != nil {
		t.Fatal(err)
	}
	_, page := getWatchlist(t, store, "?limit=1")
	if page.Total != 2 || len(page.Films) != 1 {
		t.Fatalf("total = %d, %d films, want 2 and 1", page.Total, len(page.Films))
	}
	if got := page.Films[0]; got.Name != "The Matrix" || got.AddedOn != "2024-06-01" {
		t.Errorf("updated film = %+v", got)
	}
}

func TestWatchlistPagination(t *testing.T) {
	store := newStore(t)
	var imp Import
	for i := range 120 {
		imp.Watchlist = append(imp.Watchlist, WatchlistFilm{
			Name:          fmt.Sprintf("Film %03d", i),
			LetterboxdURI: fmt.Sprintf("https://boxd.it/%d", i),
			// Day 1 to 28, then again: several films share a date.
			AddedOn: fmt.Sprintf("2024-01-%02d", i%28+1),
		})
	}
	if err := store.Import(t.Context(), imp); err != nil {
		t.Fatal(err)
	}

	_, first := getWatchlist(t, store, "")
	if first.Total != 120 || len(first.Films) != defaultPageSize {
		t.Fatalf("default page: total = %d, %d films", first.Total, len(first.Films))
	}
	if got := first.Films[0]; got.AddedOn != "2024-01-28" || got.Name != "Film 027" {
		t.Errorf("first film = %+v, want the latest date then the name order", got)
	}

	seen := map[int64]bool{}
	for offset := 0; offset < 120; offset += 50 {
		_, page := getWatchlist(t, store, fmt.Sprintf("?limit=50&offset=%d", offset))
		for _, f := range page.Films {
			if seen[f.ID] {
				t.Errorf("film %d is in two pages", f.ID)
			}
			seen[f.ID] = true
		}
	}
	if len(seen) != 120 {
		t.Errorf("pages hold %d films, want 120", len(seen))
	}

	for _, query := range []string{"?limit=0", "?limit=abc", "?offset=-1"} {
		if rec, _ := getWatchlist(t, store, query); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", query, rec.Code)
		}
	}
}

// watchlistFixture holds four films named after the rank of the day they were
// added, in reverse, of their release and of their runtime.
func watchlistFixture(t *testing.T) *Store {
	t.Helper()
	year := func(y int) *int { return &y }
	store := newStore(t)
	err := store.Import(t.Context(), Import{Watchlist: []WatchlistFilm{
		{Name: "b", Year: year(1999), LetterboxdURI: "https://boxd.it/b", AddedOn: "2024-01-03"},
		{Name: "C", Year: year(1979), LetterboxdURI: "https://boxd.it/c", AddedOn: "2024-01-01"},
		{Name: "a", Year: year(1986), LetterboxdURI: "https://boxd.it/a", AddedOn: "2024-01-02"},
		// Neither released nor timed: last whatever the direction.
		{Name: "d", LetterboxdURI: "https://boxd.it/d", AddedOn: "2024-01-02"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`
		UPDATE films SET runtime = CASE name WHEN 'a' THEN 90 WHEN 'b' THEN 120 WHEN 'C' THEN 100 END`); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestWatchlistFilters(t *testing.T) {
	store := watchlistFixture(t)
	tests := map[string]string{
		"":             "badC",
		"?decade=1980": "a",
		"?decade=1990": "b",
		"?decade=2000": "",
	}
	for query, want := range tests {
		rec, page := getWatchlist(t, store, query)
		if got := watchlistNames(page); rec.Code != http.StatusOK || got != want || page.Total != len(want) {
			t.Errorf("%q: status = %d, total = %d, films = %s, want %s", query, rec.Code, page.Total, got, want)
		}
	}

	for _, query := range []string{"?decade=1995", "?decade=x"} {
		if rec, _ := getWatchlist(t, store, query); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", query, rec.Code)
		}
	}
}

func TestWatchlistSorts(t *testing.T) {
	store := watchlistFixture(t)
	tests := map[string]string{
		"added-desc":   "badC",
		"added-asc":    "Cadb",
		"name-asc":     "abCd",
		"name-desc":    "dCba",
		"release-desc": "baCd",
		"release-asc":  "Cabd",
		"runtime-desc": "bCad",
		"runtime-asc":  "aCbd",
	}
	for sort, want := range tests {
		rec, page := getWatchlist(t, store, "?sort="+sort)
		if got := watchlistNames(page); rec.Code != http.StatusOK || got != want {
			t.Errorf("%s: status = %d, films = %s, want %s", sort, rec.Code, got, want)
		}
	}
	if len(tests) != len(watchlistOrders) {
		t.Errorf("%d sorts tested out of %d", len(tests), len(watchlistOrders))
	}

	// A sort goes along with a filter.
	if _, page := getWatchlist(t, store, "?sort=name-desc&decade=1970"); watchlistNames(page) != "C" {
		t.Errorf("sorted and filtered: films = %s, want C", watchlistNames(page))
	}
	if rec, _ := getWatchlist(t, store, "?sort=random"); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown sort: status = %d, want 400", rec.Code)
	}
}
