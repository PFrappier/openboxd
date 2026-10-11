package library

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type listsPage struct {
	Total int
	Lists []List
}

func getLists(t *testing.T, store *Store, query string) (*httptest.ResponseRecorder, listsPage) {
	t.Helper()
	rec := httptest.NewRecorder()
	NewHandler(store).Lists(rec, httptest.NewRequest(http.MethodGet, "/api/lists"+query, nil))

	var page listsPage
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
	}
	return rec, page
}

type listPage struct {
	List  List
	Films []ListFilm
}

func getList(t *testing.T, store *Store, id, query string) (*httptest.ResponseRecorder, listPage) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/lists/"+id+query, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	NewHandler(store).List(rec, req)

	var page listPage
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
	}
	return rec, page
}

func listNames(page listsPage) string {
	names := ""
	for _, l := range page.Lists {
		names += l.Name
	}
	return names
}

func listFilmNames(page listPage) string {
	names := ""
	for _, f := range page.Films {
		names += f.Name
	}
	return names
}

func TestListsEmpty(t *testing.T) {
	rec, _ := getLists(t, newStore(t), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if want := `{"total":0,"lists":[]}` + "\n"; rec.Body.String() != want {
		t.Errorf("body = %s, want %s", rec.Body, want)
	}
}

func TestLists(t *testing.T) {
	const matrix, alien, heat = "https://boxd.it/2a1m", "https://boxd.it/2awY", "https://boxd.it/2bfM"
	y1979 := 1979
	store := newStore(t)
	imp := Import{
		Watched: []WatchedFilm{{Name: "Alien", Year: &y1979, LetterboxdURI: alien, WatchedOn: "2024-01-01"}},
		Ratings: []RatedFilm{{Name: "Alien", Year: &y1979, LetterboxdURI: alien, HalfStars: 9, RatedOn: "2024-01-01"}},
		Likes:   []LikedFilm{{Name: "Alien", Year: &y1979, LetterboxdURI: alien, LikedOn: "2024-01-01"}},
		Lists: []ImportedList{
			{
				Name: "Top", LetterboxdURI: "https://boxd.it/top", CreatedOn: "2024-02-01", Description: "The best.",
				Films: []ListFilm{
					{Name: "The Matrix", LetterboxdURI: matrix},
					{Name: "Alien", Year: &y1979, LetterboxdURI: alien},
					// Listed twice: only its first position counts.
					{Name: "The Matrix", LetterboxdURI: matrix},
				},
			},
			{Name: "Empty", LetterboxdURI: "https://boxd.it/empty", CreatedOn: "2024-03-01"},
		},
	}
	// Importing again doesn't duplicate the lists, nor their films.
	for range 2 {
		if err := store.Import(t.Context(), imp); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetMetadata(t.Context(), 1, Metadata{TMDBID: 348, PosterPath: "/alien.jpg", Runtime: 117}); err != nil {
		t.Fatal(err)
	}

	rec, _ := getLists(t, store, "")
	want := `{"total":2,"lists":[` +
		`{"id":2,"name":"Empty","letterboxdUri":"https://boxd.it/empty","createdOn":"2024-03-01",` +
		`"description":"","filmCount":0,"posters":[]},` +
		`{"id":1,"name":"Top","letterboxdUri":"https://boxd.it/top","createdOn":"2024-02-01",` +
		`"description":"The best.","filmCount":2,"posters":[null,"/alien.jpg"]}]}` + "\n"
	if rec.Body.String() != want {
		t.Errorf("body = %s\nwant %s", rec.Body, want)
	}

	rec, _ = getList(t, store, "1", "")
	want = `{"list":{"id":1,"name":"Top","letterboxdUri":"https://boxd.it/top","createdOn":"2024-02-01",` +
		`"description":"The best.","filmCount":2,"posters":[null,"/alien.jpg"]},"films":[` +
		`{"id":2,"name":"The Matrix","year":null,"letterboxdUri":"https://boxd.it/2a1m","position":1,` +
		`"rating":null,"liked":false,"posterPath":null},` +
		`{"id":1,"name":"Alien","year":1979,"letterboxdUri":"https://boxd.it/2awY","position":2,` +
		`"rating":4.5,"liked":true,"posterPath":"/alien.jpg"}]}` + "\n"
	if rec.Body.String() != want {
		t.Errorf("body = %s\nwant %s", rec.Body, want)
	}
	if rec, page := getList(t, store, "2", ""); rec.Code != http.StatusOK || page.Films == nil || len(page.Films) != 0 {
		t.Errorf("empty list: status = %d, body = %s", rec.Code, rec.Body)
	}
	if _, page := getWatched(t, store, ""); page.Total != 1 {
		t.Errorf("films of the lists count as watched: total = %d, want 1", page.Total)
	}

	// A list changed on Letterboxd is replaced, films included.
	imp.Lists = []ImportedList{{
		Name: "Top 2", LetterboxdURI: "https://boxd.it/top", CreatedOn: "2024-02-01",
		Films: []ListFilm{
			{Name: "Heat", LetterboxdURI: heat},
			{Name: "The Matrix", LetterboxdURI: matrix},
		},
	}}
	if err := store.Import(t.Context(), imp); err != nil {
		t.Fatal(err)
	}
	_, page := getList(t, store, "1", "")
	if page.List.Name != "Top 2" || page.List.Description != "" || page.List.FilmCount != 2 || listFilmNames(page) != "HeatThe Matrix" {
		t.Errorf("replaced list = %+v, films = %s", page.List, listFilmNames(page))
	}
	// The list missing from the later export stays.
	if _, lists := getLists(t, store, ""); lists.Total != 2 {
		t.Errorf("total = %d lists, want 2", lists.Total)
	}
}

func TestListPagination(t *testing.T) {
	store := newStore(t)
	list := ImportedList{Name: "Long", LetterboxdURI: "https://boxd.it/long", CreatedOn: "2024-01-01"}
	for i := range 120 {
		list.Films = append(list.Films, ListFilm{
			// Named against the order of the list, which isn't the one of the names.
			Name:          fmt.Sprintf("Film %03d", 119-i),
			LetterboxdURI: fmt.Sprintf("https://boxd.it/%d", i),
		})
	}
	if err := store.Import(t.Context(), Import{Lists: []ImportedList{list}}); err != nil {
		t.Fatal(err)
	}

	_, first := getList(t, store, "1", "")
	if first.List.FilmCount != 120 || len(first.Films) != defaultPageSize || len(first.List.Posters) != listPosters {
		t.Fatalf("default page: %d films out of %d, %d posters", len(first.Films), first.List.FilmCount, len(first.List.Posters))
	}
	position := 0
	for offset := 0; offset < 120; offset += 50 {
		_, page := getList(t, store, "1", fmt.Sprintf("?limit=50&offset=%d", offset))
		for _, f := range page.Films {
			position++
			if f.Position != position || f.Name != fmt.Sprintf("Film %03d", 120-position) {
				t.Fatalf("film %d of the list = %+v", position, f)
			}
		}
	}
	if position != 120 {
		t.Errorf("pages hold %d films, want 120", position)
	}

	for _, query := range []string{"?limit=0", "?limit=abc", "?offset=-1"} {
		if rec, _ := getList(t, store, "1", query); rec.Code != http.StatusBadRequest {
			t.Errorf("list, %s: status = %d, want 400", query, rec.Code)
		}
		if rec, _ := getLists(t, store, query); rec.Code != http.StatusBadRequest {
			t.Errorf("lists, %s: status = %d, want 400", query, rec.Code)
		}
	}
	if _, page := getLists(t, store, "?limit=1&offset=1"); page.Total != 1 || len(page.Lists) != 0 {
		t.Errorf("past the last list: total = %d, %d lists", page.Total, len(page.Lists))
	}
}

func TestListNotFound(t *testing.T) {
	store := newStore(t)
	for _, id := range []string{"1", "abc"} {
		rec, _ := getList(t, store, id, "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", id, rec.Code)
		}
	}
}

func TestListsSorts(t *testing.T) {
	store := newStore(t)
	films := func(n int) []ListFilm {
		var films []ListFilm
		for i := range n {
			films = append(films, ListFilm{Name: "Film", LetterboxdURI: fmt.Sprintf("https://boxd.it/%d", i)})
		}
		return films
	}
	// Named after the rank of the day they were made, in reverse of their size.
	err := store.Import(t.Context(), Import{Lists: []ImportedList{
		{Name: "b", LetterboxdURI: "https://boxd.it/b", CreatedOn: "2024-01-02", Films: films(2)},
		{Name: "C", LetterboxdURI: "https://boxd.it/c", CreatedOn: "2024-01-03", Films: films(1)},
		{Name: "a", LetterboxdURI: "https://boxd.it/a", CreatedOn: "2024-01-01", Films: films(3)},
	}})
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]string{
		"":             "Cba",
		"created-desc": "Cba",
		"created-asc":  "abC",
		"name-asc":     "abC",
		"name-desc":    "Cba",
		"films-desc":   "abC",
		"films-asc":    "Cba",
	}
	for sort, want := range tests {
		query := ""
		if sort != "" {
			query = "?sort=" + sort
		}
		rec, page := getLists(t, store, query)
		if got := listNames(page); rec.Code != http.StatusOK || got != want {
			t.Errorf("%q: status = %d, lists = %s, want %s", sort, rec.Code, got, want)
		}
	}
	if len(tests)-1 != len(listOrders) {
		t.Errorf("%d sorts tested out of %d", len(tests)-1, len(listOrders))
	}
	if rec, _ := getLists(t, store, "?sort=random"); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown sort: status = %d, want 400", rec.Code)
	}
}
