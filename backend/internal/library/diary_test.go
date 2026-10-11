package library

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type diaryPage struct {
	Total   int
	Years   []int
	Entries []DiaryEntry
}

func getDiary(t *testing.T, store *Store, query string) (*httptest.ResponseRecorder, diaryPage) {
	t.Helper()
	rec := httptest.NewRecorder()
	NewHandler(store).Diary(rec, httptest.NewRequest(http.MethodGet, "/api/diary"+query, nil))

	var page diaryPage
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
	}
	return rec, page
}

func TestDiaryEmpty(t *testing.T) {
	rec, _ := getDiary(t, newStore(t), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if want := `{"total":0,"years":[],"entries":[]}` + "\n"; rec.Body.String() != want {
		t.Errorf("body = %s, want %s", rec.Body, want)
	}
}

func TestDiary(t *testing.T) {
	const matrix, alien = "https://boxd.it/2a1m", "https://boxd.it/2awY"
	store := newStore(t)
	imp := Import{
		Likes: []LikedFilm{{Name: "The Matrix", LetterboxdURI: matrix, LikedOn: "2024-01-02"}},
		Diary: []LoggedFilm{
			{Name: "The Matrix", LetterboxdURI: matrix, EntryURI: "https://boxd.it/e1", WatchedOn: "2024-01-02", HalfStars: 8},
			{Name: "Alien", LetterboxdURI: alien, EntryURI: "https://boxd.it/e2", WatchedOn: "2024-03-05"},
			// Watched the same day as Alien, logged after it.
			{Name: "The Matrix", LetterboxdURI: matrix, EntryURI: "https://boxd.it/e3", WatchedOn: "2024-03-05", HalfStars: 9, Rewatch: true},
		},
	}
	// Importing again doesn't duplicate the entries.
	for range 2 {
		if err := store.Import(t.Context(), imp); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetMetadata(t.Context(), 1, Metadata{TMDBID: 603, PosterPath: "/matrix.jpg"}); err != nil {
		t.Fatal(err)
	}

	rec, _ := getDiary(t, store, "")
	want := `{"total":3,"years":[2024],"entries":[` +
		`{"id":3,"watchedOn":"2024-03-05","rating":4.5,"rewatch":true,"reviewed":false,` +
		`"film":{"id":1,"name":"The Matrix","year":null,"posterPath":"/matrix.jpg","liked":true}},` +
		`{"id":2,"watchedOn":"2024-03-05","rating":null,"rewatch":false,"reviewed":false,` +
		`"film":{"id":2,"name":"Alien","year":null,"posterPath":null,"liked":false}},` +
		`{"id":1,"watchedOn":"2024-01-02","rating":4,"rewatch":false,"reviewed":false,` +
		`"film":{"id":1,"name":"The Matrix","year":null,"posterPath":"/matrix.jpg","liked":true}}]}` + "\n"
	if rec.Body.String() != want {
		t.Errorf("body = %s\nwant %s", rec.Body, want)
	}

	// The film lists its own viewings, and isn't rated by them.
	film, err := store.Film(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(film.Diary) != 2 || film.Diary[0].ID != 3 || film.Diary[1].ID != 1 {
		t.Errorf("viewings of the film = %+v, want entries 3 then 1", film.Diary)
	}
	if film.Rating != nil {
		t.Errorf("rating of the film = %v, want none", *film.Rating)
	}

	// An entry that changed on Letterboxd is updated.
	imp.Diary = []LoggedFilm{
		{Name: "Alien", LetterboxdURI: alien, EntryURI: "https://boxd.it/e2", WatchedOn: "2024-03-06", HalfStars: 10},
	}
	if err := store.Import(t.Context(), imp); err != nil {
		t.Fatal(err)
	}
	_, page := getDiary(t, store, "?limit=1")
	if page.Total != 3 || len(page.Entries) != 1 {
		t.Fatalf("total = %d, %d entries, want 3 and 1", page.Total, len(page.Entries))
	}
	if got := page.Entries[0]; got.ID != 2 || got.WatchedOn != "2024-03-06" || got.Rating == nil || *got.Rating != 5 {
		t.Errorf("updated entry = %+v", got)
	}
}

func TestDiaryPagination(t *testing.T) {
	store := newStore(t)
	var imp Import
	for i := range 120 {
		imp.Diary = append(imp.Diary, LoggedFilm{
			Name:          fmt.Sprintf("Film %03d", i),
			LetterboxdURI: fmt.Sprintf("https://boxd.it/%d", i),
			EntryURI:      fmt.Sprintf("https://boxd.it/e%d", i),
			// Day 1 to 28, then again: several entries share a date.
			WatchedOn: fmt.Sprintf("2024-01-%02d", i%28+1),
		})
	}
	if err := store.Import(t.Context(), imp); err != nil {
		t.Fatal(err)
	}

	_, first := getDiary(t, store, "")
	if first.Total != 120 || len(first.Entries) != defaultPageSize {
		t.Fatalf("default page: total = %d, %d entries", first.Total, len(first.Entries))
	}

	seen := map[int64]bool{}
	for offset := 0; offset < 120; offset += 50 {
		_, page := getDiary(t, store, fmt.Sprintf("?limit=50&offset=%d", offset))
		for _, e := range page.Entries {
			if seen[e.ID] {
				t.Errorf("entry %d is in two pages", e.ID)
			}
			seen[e.ID] = true
		}
	}
	if len(seen) != 120 {
		t.Errorf("pages hold %d entries, want 120", len(seen))
	}

	for _, query := range []string{"?limit=0", "?limit=abc", "?offset=-1"} {
		if rec, _ := getDiary(t, store, query); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", query, rec.Code)
		}
	}
}

// diaryFixture logs five viewings over two years; the names of the entries
// returned by entryNames are "<film>@<day>".
func diaryFixture(t *testing.T) *Store {
	t.Helper()
	const matrix, alien, heat = "https://boxd.it/2a1m", "https://boxd.it/2awY", "https://boxd.it/2bfM"
	y1979, y1995, y1999 := 1979, 1995, 1999
	store := newStore(t)
	err := store.Import(t.Context(), Import{
		Likes: []LikedFilm{{Name: "Heat", Year: &y1995, LetterboxdURI: heat, LikedOn: "2023-12-31"}},
		Diary: []LoggedFilm{
			{Name: "The Matrix", Year: &y1999, LetterboxdURI: matrix, EntryURI: "e1", WatchedOn: "2023-05-01", HalfStars: 8},
			{Name: "Heat", Year: &y1995, LetterboxdURI: heat, EntryURI: "e2", WatchedOn: "2023-12-31", HalfStars: 10},
			{Name: "alien", Year: &y1979, LetterboxdURI: alien, EntryURI: "e3", WatchedOn: "2024-01-01"},
			{Name: "The Matrix", Year: &y1999, LetterboxdURI: matrix, EntryURI: "e4", WatchedOn: "2024-02-02", HalfStars: 8, Rewatch: true},
			{Name: "Heat", Year: &y1995, LetterboxdURI: heat, EntryURI: "e5", WatchedOn: "2024-02-02", HalfStars: 9, Rewatch: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func entryNames(page diaryPage) string {
	names := ""
	for i, e := range page.Entries {
		if i > 0 {
			names += " "
		}
		names += e.Film.Name + "@" + e.WatchedOn[2:]
	}
	return names
}

func TestDiaryFilters(t *testing.T) {
	store := diaryFixture(t)
	tests := map[string]string{
		"":                                   "Heat@24-02-02 The Matrix@24-02-02 alien@24-01-01 Heat@23-12-31 The Matrix@23-05-01",
		"?year=2023":                         "Heat@23-12-31 The Matrix@23-05-01",
		"?year=2024":                         "Heat@24-02-02 The Matrix@24-02-02 alien@24-01-01",
		"?year=2022":                         "",
		"?rating=4":                          "The Matrix@24-02-02 The Matrix@23-05-01",
		"?rating=4.5":                        "Heat@24-02-02",
		"?rating=none":                       "alien@24-01-01",
		"?liked=true":                        "Heat@24-02-02 Heat@23-12-31",
		"?rewatch=true":                      "Heat@24-02-02 The Matrix@24-02-02",
		"?year=2024&rating=4&rewatch=true":   "The Matrix@24-02-02",
		"?liked=true&rewatch=true&year=2023": "",
	}
	for query, want := range tests {
		rec, page := getDiary(t, store, query)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, body = %s", query, rec.Code, rec.Body)
			continue
		}
		if got := entryNames(page); got != want {
			t.Errorf("%s:\n got %s\nwant %s", query, got, want)
		}
		if page.Total != len(page.Entries) {
			t.Errorf("%s: total = %d for %d entries", query, page.Total, len(page.Entries))
		}
		// The years are those of the whole diary, whatever the filter.
		if len(page.Years) != 2 || page.Years[0] != 2024 || page.Years[1] != 2023 {
			t.Errorf("%s: years = %v, want [2024 2023]", query, page.Years)
		}
	}

	for _, query := range []string{"?year=abc", "?year=24", "?rating=6", "?rating=3.2", "?liked=1", "?rewatch=yes"} {
		if rec, _ := getDiary(t, store, query); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", query, rec.Code)
		}
	}
}

func TestDiarySorts(t *testing.T) {
	store := diaryFixture(t)
	tests := map[string]string{
		"watched-desc": "Heat@24-02-02 The Matrix@24-02-02 alien@24-01-01 Heat@23-12-31 The Matrix@23-05-01",
		"watched-asc":  "The Matrix@23-05-01 Heat@23-12-31 alien@24-01-01 The Matrix@24-02-02 Heat@24-02-02",
		// The viewing without a rating comes last in both directions.
		"rating-desc":  "Heat@23-12-31 Heat@24-02-02 The Matrix@24-02-02 The Matrix@23-05-01 alien@24-01-01",
		"rating-asc":   "The Matrix@24-02-02 The Matrix@23-05-01 Heat@24-02-02 Heat@23-12-31 alien@24-01-01",
		"name-asc":     "alien@24-01-01 Heat@24-02-02 Heat@23-12-31 The Matrix@24-02-02 The Matrix@23-05-01",
		"name-desc":    "The Matrix@24-02-02 The Matrix@23-05-01 Heat@24-02-02 Heat@23-12-31 alien@24-01-01",
		"release-desc": "The Matrix@24-02-02 The Matrix@23-05-01 Heat@24-02-02 Heat@23-12-31 alien@24-01-01",
		"release-asc":  "alien@24-01-01 Heat@24-02-02 Heat@23-12-31 The Matrix@24-02-02 The Matrix@23-05-01",
	}
	for sort, want := range tests {
		rec, page := getDiary(t, store, "?sort="+sort)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, body = %s", sort, rec.Code, rec.Body)
			continue
		}
		if got := entryNames(page); got != want {
			t.Errorf("%s:\n got %s\nwant %s", sort, got, want)
		}
	}
	if len(tests) != len(diaryOrders) {
		t.Errorf("%d sorts tested out of %d", len(tests), len(diaryOrders))
	}

	if rec, _ := getDiary(t, store, "?sort=random"); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown sort: status = %d, want 400", rec.Code)
	}
}
