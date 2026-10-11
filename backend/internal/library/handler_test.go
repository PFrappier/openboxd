package library

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
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

func TestWatchedFilters(t *testing.T) {
	store := newStore(t)
	year := func(y int) *int { return &y }
	film := func(name string, y *int) WatchedFilm {
		return WatchedFilm{Name: name, Year: y, LetterboxdURI: "https://boxd.it/" + name, WatchedOn: "2024-01-01"}
	}
	alien, matrix, undated := film("Alien", year(1979)), film("The Matrix", year(1999)), film("Undated", nil)
	err := store.Import(t.Context(), Import{
		Watched: []WatchedFilm{alien, matrix, undated, film("Aliens", year(1986))},
		Ratings: []RatedFilm{
			{Name: alien.Name, Year: alien.Year, LetterboxdURI: alien.LetterboxdURI, HalfStars: 9, RatedOn: "2024-01-01"},
			{Name: matrix.Name, Year: matrix.Year, LetterboxdURI: matrix.LetterboxdURI, HalfStars: 8, RatedOn: "2024-01-01"},
		},
		Likes: []LikedFilm{
			{Name: alien.Name, Year: alien.Year, LetterboxdURI: alien.LetterboxdURI, LikedOn: "2024-01-01"},
			// Liked without being watched: never listed.
			{Name: "Heat", LetterboxdURI: "https://boxd.it/Heat", LikedOn: "2024-01-01"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		query string
		want  []string
	}{
		{"", []string{"Alien", "Aliens", "The Matrix", "Undated"}},
		{"?rating=4.5", []string{"Alien"}},
		{"?rating=4", []string{"The Matrix"}},
		{"?rating=none", []string{"Aliens", "Undated"}},
		{"?liked=true", []string{"Alien"}},
		{"?decade=1980", []string{"Aliens"}},
		{"?decade=1970&liked=true&rating=4.5", []string{"Alien"}},
		{"?decade=1990&liked=true", nil},
	}
	for _, test := range tests {
		rec, page := getWatched(t, store, test.query)
		var got []string
		for _, f := range page.Films {
			got = append(got, f.Name)
		}
		if rec.Code != http.StatusOK || !slices.Equal(got, test.want) || page.Total != len(test.want) {
			t.Errorf("%q: status = %d, total = %d, films = %v, want %v", test.query, rec.Code, page.Total, got, test.want)
		}
	}

	// The total counts all the films kept, not just those of the page.
	if _, page := getWatched(t, store, "?rating=none&limit=1&offset=1"); page.Total != 2 || len(page.Films) != 1 {
		t.Errorf("second page of a filter: total = %d, %d films", page.Total, len(page.Films))
	}

	for _, query := range []string{"?rating=6", "?rating=0", "?rating=4.2", "?rating=x", "?liked=yes", "?decade=1995", "?decade=x"} {
		if rec, _ := getWatched(t, store, query); rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status = %d, want %d", query, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestWatchedSorts(t *testing.T) {
	store := newStore(t)
	year := func(y int) *int { return &y }
	// Named after the rank of their date, of their release and of their rating.
	films := []WatchedFilm{
		{Name: "b", Year: year(1999), LetterboxdURI: "https://boxd.it/b", WatchedOn: "2024-01-03"},
		{Name: "C", Year: year(1979), LetterboxdURI: "https://boxd.it/c", WatchedOn: "2024-01-01"},
		{Name: "a", Year: year(1986), LetterboxdURI: "https://boxd.it/a", WatchedOn: "2024-01-02"},
		// Neither released, rated nor timed: last whatever the direction.
		{Name: "d", LetterboxdURI: "https://boxd.it/d", WatchedOn: "2024-01-02"},
	}
	rated := func(f WatchedFilm, halfStars int, on string) RatedFilm {
		return RatedFilm{Name: f.Name, Year: f.Year, LetterboxdURI: f.LetterboxdURI, HalfStars: halfStars, RatedOn: on}
	}
	err := store.Import(t.Context(), Import{
		Watched: films,
		Ratings: []RatedFilm{
			rated(films[0], 6, "2024-02-01"),
			rated(films[1], 10, "2024-02-03"),
			rated(films[2], 6, "2024-02-02"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`
		UPDATE films SET runtime = CASE name WHEN 'a' THEN 90 WHEN 'b' THEN 120 WHEN 'C' THEN 100 END`); err != nil {
		t.Fatal(err)
	}

	tests := []struct{ sort, want string }{
		{"", "badC"},
		{"added-desc", "badC"},
		{"added-asc", "Cadb"},
		{"name-asc", "abCd"},
		{"name-desc", "dCba"},
		{"release-desc", "baCd"},
		{"release-asc", "Cabd"},
		// Films rated the same are in the order of their names.
		{"rating-desc", "Cabd"},
		{"rating-asc", "abCd"},
		{"rated-desc", "Cabd"},
		{"rated-asc", "baCd"},
		{"runtime-desc", "bCad"},
		{"runtime-asc", "aCbd"},
	}
	for _, test := range tests {
		rec, page := getWatched(t, store, "?sort="+test.sort)
		var got string
		for _, f := range page.Films {
			got += f.Name
		}
		if rec.Code != http.StatusOK || got != test.want {
			t.Errorf("sort %q: status = %d, films = %s, want %s", test.sort, rec.Code, got, test.want)
		}
	}

	// A sort goes along with a filter.
	if _, page := getWatched(t, store, "?sort=name-desc&rating=3"); len(page.Films) != 2 || page.Films[0].Name != "b" {
		t.Errorf("sorted and filtered: films = %+v, want b then a", page.Films)
	}
	if rec, _ := getWatched(t, store, "?sort=name"); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown sort: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestWatchedDetails(t *testing.T) {
	store := newStore(t)
	err := store.Import(t.Context(), Import{
		Watched: []WatchedFilm{
			{Name: "The Matrix", LetterboxdURI: "https://boxd.it/2a1m", WatchedOn: "2024-01-02"},
			{Name: "Not on TMDB", LetterboxdURI: "https://boxd.it/0", WatchedOn: "2024-01-01"},
		},
		Ratings: []RatedFilm{
			{Name: "The Matrix", LetterboxdURI: "https://boxd.it/2a1m", HalfStars: 9, RatedOn: "2024-01-03"},
		},
		Likes: []LikedFilm{
			{Name: "The Matrix", LetterboxdURI: "https://boxd.it/2a1m", LikedOn: "2024-01-03"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, page := getWatched(t, store, "")
	err = store.SetMetadata(t.Context(), page.Films[0].ID, Metadata{
		TMDBID:     603,
		Overview:   "Un pirate informatique découvre la Matrice.",
		PosterPath: "/matrix.jpg",
		Runtime:    136,
		// Stored in this order, which isn't the alphabetical one.
		Directors: []Person{{TMDBID: 9340, Name: "Lilly Wachowski"}, {TMDBID: 9339, Name: "Lana Wachowski"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	rec, _ := getWatched(t, store, "")
	want := `{"total":2,"films":[` +
		`{"id":1,"name":"The Matrix","year":null,"letterboxdUri":"https://boxd.it/2a1m","watchedOn":"2024-01-02",` +
		`"rating":4.5,"liked":true,"reviewed":false,` +
		`"overview":"Un pirate informatique découvre la Matrice.","posterPath":"/matrix.jpg","runtime":136,` +
		`"directors":["Lilly Wachowski","Lana Wachowski"]},` +
		`{"id":2,"name":"Not on TMDB","year":null,"letterboxdUri":"https://boxd.it/0","watchedOn":"2024-01-01",` +
		`"rating":null,"liked":false,"reviewed":false,` +
		`"overview":null,"posterPath":null,"runtime":null,"directors":[]}]}` + "\n"
	if rec.Body.String() != want {
		t.Errorf("body = %s\nwant %s", rec.Body, want)
	}
}

func getFilm(store *Store, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/films/"+id, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	NewHandler(store).Film(rec, req)
	return rec
}

func TestFilm(t *testing.T) {
	store := newStore(t)
	err := store.AddWatched(t.Context(), []WatchedFilm{
		{Name: "The Matrix", LetterboxdURI: "https://boxd.it/2a1m", WatchedOn: "2024-01-02"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Before its details are fetched.
	rec := getFilm(store, "1")
	want := `{"id":1,"name":"The Matrix","year":null,"letterboxdUri":"https://boxd.it/2a1m","watchedOn":"2024-01-02",` +
		`"rating":null,"liked":false,"watchlistedOn":null,` +
		`"tmdbId":null,"overview":null,"posterPath":null,"runtime":null,"directors":[],"diary":[],"reviews":[]}` + "\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Errorf("status = %d, body = %s\nwant %s", rec.Code, rec.Body, want)
	}

	err = store.SetMetadata(t.Context(), 1, Metadata{
		TMDBID:     603,
		Overview:   "Un pirate informatique découvre la Matrice.",
		PosterPath: "/matrix.jpg",
		Runtime:    136,
		Directors:  []Person{{TMDBID: 9340, Name: "Lilly Wachowski"}, {TMDBID: 9339, Name: "Lana Wachowski"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	rec = getFilm(store, "1")
	want = `{"id":1,"name":"The Matrix","year":null,"letterboxdUri":"https://boxd.it/2a1m","watchedOn":"2024-01-02",` +
		`"rating":null,"liked":false,"watchlistedOn":null,` +
		`"tmdbId":603,"overview":"Un pirate informatique découvre la Matrice.","posterPath":"/matrix.jpg","runtime":136,` +
		`"directors":["Lilly Wachowski","Lana Wachowski"],"diary":[],"reviews":[]}` + "\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Errorf("status = %d, body = %s\nwant %s", rec.Code, rec.Body, want)
	}
}

func TestFilmRatedAndLiked(t *testing.T) {
	store := newStore(t)
	matrix := func(halfStars int) Import {
		return Import{
			Ratings: []RatedFilm{
				{Name: "The Matrix", LetterboxdURI: "https://boxd.it/2a1m", HalfStars: halfStars, RatedOn: "2024-01-03"},
			},
			// Liking a film doesn't require having watched it.
			Likes: []LikedFilm{
				{Name: "Alien", LetterboxdURI: "https://boxd.it/2awY", LikedOn: "2024-01-04"},
			},
		}
	}
	// Importing again replaces the rating instead of failing or duplicating.
	for _, halfStars := range []int{7, 10} {
		if err := store.Import(t.Context(), matrix(halfStars)); err != nil {
			t.Fatal(err)
		}
	}

	rec := getFilm(store, "1")
	want := `{"id":1,"name":"The Matrix","year":null,"letterboxdUri":"https://boxd.it/2a1m","watchedOn":null,` +
		`"rating":5,"liked":false,"watchlistedOn":null,` +
		`"tmdbId":null,"overview":null,"posterPath":null,"runtime":null,"directors":[],"diary":[],"reviews":[]}` + "\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Errorf("status = %d, body = %s\nwant %s", rec.Code, rec.Body, want)
	}

	rec = getFilm(store, "2")
	want = `{"id":2,"name":"Alien","year":null,"letterboxdUri":"https://boxd.it/2awY","watchedOn":null,` +
		`"rating":null,"liked":true,"watchlistedOn":null,` +
		`"tmdbId":null,"overview":null,"posterPath":null,"runtime":null,"directors":[],"diary":[],"reviews":[]}` + "\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Errorf("status = %d, body = %s\nwant %s", rec.Code, rec.Body, want)
	}

	if _, page := getWatched(t, store, ""); page.Total != 0 {
		t.Errorf("rated and liked films count as watched: total = %d", page.Total)
	}
}

func TestImportRejectsInvalidRating(t *testing.T) {
	store := newStore(t)
	err := store.Import(t.Context(), Import{
		Watched: []WatchedFilm{{Name: "Alien", LetterboxdURI: "https://boxd.it/2awY", WatchedOn: "2024-01-01"}},
		Ratings: []RatedFilm{{Name: "Alien", LetterboxdURI: "https://boxd.it/2awY", HalfStars: 11, RatedOn: "2024-01-01"}},
	})
	if err == nil {
		t.Fatal("a rating of 11 half stars was accepted")
	}
	// All or none: the watched film of the same import isn't kept either.
	if _, page := getWatched(t, store, ""); page.Total != 0 {
		t.Errorf("total = %d after a failed import, want 0", page.Total)
	}
}

func TestFilmNotFound(t *testing.T) {
	store := newStore(t)
	for _, id := range []string{"1", "abc", "-1", ""} {
		if rec := getFilm(store, id); rec.Code != http.StatusNotFound {
			t.Errorf("film %q: status = %d, want 404", id, rec.Code)
		}
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
