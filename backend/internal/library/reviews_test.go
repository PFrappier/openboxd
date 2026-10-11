package library

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type reviewsPage struct {
	Total   int
	Reviews []ReviewEntry
}

func getReviews(t *testing.T, store *Store, query string) (*httptest.ResponseRecorder, reviewsPage) {
	t.Helper()
	rec := httptest.NewRecorder()
	NewHandler(store).Reviews(rec, httptest.NewRequest(http.MethodGet, "/api/reviews"+query, nil))

	var page reviewsPage
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
	}
	return rec, page
}

// reviewNames returns the films of the reviews of a page, in its order.
func reviewNames(page reviewsPage) string {
	names := ""
	for _, r := range page.Reviews {
		names += r.Film.Name + " "
	}
	return names
}

func TestReviewsEmpty(t *testing.T) {
	rec, _ := getReviews(t, newStore(t), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if want := `{"total":0,"reviews":[]}` + "\n"; rec.Body.String() != want {
		t.Errorf("body = %s, want %s", rec.Body, want)
	}
}

func TestReviews(t *testing.T) {
	const matrix, alien = "https://boxd.it/2a1m", "https://boxd.it/2awY"
	store := newStore(t)
	imp := Import{
		Likes: []LikedFilm{{Name: "The Matrix", LetterboxdURI: matrix, LikedOn: "2024-01-02"}},
		Reviews: []ReviewedFilm{
			{Name: "The Matrix", LetterboxdURI: matrix, ReviewURI: "https://boxd.it/r1",
				ReviewedOn: "2024-01-03", WatchedOn: "2024-01-02", HalfStars: 8, Text: "Whoa."},
			// Written without logging a viewing, nor rating the film.
			{Name: "Alien", LetterboxdURI: alien, ReviewURI: "https://boxd.it/r2",
				ReviewedOn: "2024-03-05", Text: "In <i>space</i>…\nno one can hear you."},
			{Name: "The Matrix", LetterboxdURI: matrix, ReviewURI: "https://boxd.it/r3",
				ReviewedOn: "2024-03-05", WatchedOn: "2024-03-05", HalfStars: 9, Rewatch: true, Text: "Still whoa."},
		},
	}
	// Importing again doesn't duplicate the reviews.
	for range 2 {
		if err := store.Import(t.Context(), imp); err != nil {
			t.Fatal(err)
		}
	}

	rec, _ := getReviews(t, store, "")
	want := `{"total":3,"reviews":[` +
		`{"id":3,"reviewedOn":"2024-03-05","watchedOn":"2024-03-05","rating":4.5,"rewatch":true,"text":"Still whoa.",` +
		`"film":{"id":1,"name":"The Matrix","year":null,"posterPath":null,"liked":true}},` +
		`{"id":2,"reviewedOn":"2024-03-05","watchedOn":null,"rating":null,"rewatch":false,` +
		`"text":"In \u003ci\u003espace\u003c/i\u003e…\nno one can hear you.",` +
		`"film":{"id":2,"name":"Alien","year":null,"posterPath":null,"liked":false}},` +
		`{"id":1,"reviewedOn":"2024-01-03","watchedOn":"2024-01-02","rating":4,"rewatch":false,"text":"Whoa.",` +
		`"film":{"id":1,"name":"The Matrix","year":null,"posterPath":null,"liked":true}}]}` + "\n"
	if rec.Body.String() != want {
		t.Errorf("body = %s\nwant %s", rec.Body, want)
	}

	// The film lists its own reviews, and isn't rated by them.
	film, err := store.Film(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(film.Reviews) != 2 || film.Reviews[0].ID != 3 || film.Reviews[1].ID != 1 {
		t.Errorf("reviews of the film = %+v, want reviews 3 then 1", film.Reviews)
	}
	if film.Rating != nil {
		t.Errorf("rating of the film = %v, want none", *film.Rating)
	}

	// Only the viewing the review was written along with is told as reviewed.
	imp.Watched = []WatchedFilm{
		{Name: "The Matrix", LetterboxdURI: matrix, WatchedOn: "2024-01-02"},
		{Name: "Heat", LetterboxdURI: "https://boxd.it/2bfM", WatchedOn: "2024-01-01"},
	}
	imp.Diary = []LoggedFilm{
		{Name: "The Matrix", LetterboxdURI: matrix, EntryURI: "https://boxd.it/r1", WatchedOn: "2024-01-02"},
		{Name: "The Matrix", LetterboxdURI: matrix, EntryURI: "https://boxd.it/e2", WatchedOn: "2024-02-02"},
	}
	if err := store.Import(t.Context(), imp); err != nil {
		t.Fatal(err)
	}
	_, diary := getDiary(t, store, "")
	if len(diary.Entries) != 2 || diary.Entries[0].Reviewed || !diary.Entries[1].Reviewed {
		t.Errorf("diary = %+v, want only the earliest viewing reviewed", diary.Entries)
	}
	if film, err = store.Film(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	if len(film.Diary) != 2 || film.Diary[0].Reviewed || !film.Diary[1].Reviewed {
		t.Errorf("viewings of the film = %+v, want only the earliest reviewed", film.Diary)
	}
	// A watched film is, as soon as it has a review.
	_, watched := getWatched(t, store, "")
	if len(watched.Films) != 2 || !watched.Films[0].Reviewed || watched.Films[1].Reviewed {
		t.Errorf("watched films = %+v, want only The Matrix reviewed", watched.Films)
	}

	// A review that changed on Letterboxd is updated.
	imp.Reviews = []ReviewedFilm{
		{Name: "Alien", LetterboxdURI: alien, ReviewURI: "https://boxd.it/r2",
			ReviewedOn: "2024-03-06", WatchedOn: "2024-03-04", HalfStars: 10, Text: "Perfect."},
	}
	if err := store.Import(t.Context(), imp); err != nil {
		t.Fatal(err)
	}
	_, page := getReviews(t, store, "?limit=1")
	if page.Total != 3 || len(page.Reviews) != 1 {
		t.Fatalf("total = %d, %d reviews, want 3 and 1", page.Total, len(page.Reviews))
	}
	got := page.Reviews[0]
	if got.ID != 2 || got.ReviewedOn != "2024-03-06" || got.Text != "Perfect." ||
		got.WatchedOn == nil || *got.WatchedOn != "2024-03-04" || got.Rating == nil || *got.Rating != 5 {
		t.Errorf("updated review = %+v", got)
	}
}

func TestReviewsPagination(t *testing.T) {
	store := newStore(t)
	var imp Import
	for i := range 120 {
		imp.Reviews = append(imp.Reviews, ReviewedFilm{
			Name:          fmt.Sprintf("Film %03d", i),
			LetterboxdURI: fmt.Sprintf("https://boxd.it/%d", i),
			ReviewURI:     fmt.Sprintf("https://boxd.it/r%d", i),
			// Day 1 to 28, then again: several reviews share a date.
			ReviewedOn: fmt.Sprintf("2024-01-%02d", i%28+1),
			Text:       "Fine.",
		})
	}
	if err := store.Import(t.Context(), imp); err != nil {
		t.Fatal(err)
	}

	_, first := getReviews(t, store, "")
	if first.Total != 120 || len(first.Reviews) != defaultPageSize {
		t.Fatalf("default page: total = %d, %d reviews", first.Total, len(first.Reviews))
	}

	seen := map[int64]bool{}
	for offset := 0; offset < 120; offset += 50 {
		_, page := getReviews(t, store, fmt.Sprintf("?limit=50&offset=%d", offset))
		for _, r := range page.Reviews {
			if seen[r.ID] {
				t.Errorf("review %d is in two pages", r.ID)
			}
			seen[r.ID] = true
		}
	}
	if len(seen) != 120 {
		t.Errorf("pages hold %d reviews, want 120", len(seen))
	}

	for _, query := range []string{"?limit=0", "?limit=abc", "?offset=-1"} {
		if rec, _ := getReviews(t, store, query); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", query, rec.Code)
		}
	}
}

func TestReviewsSorts(t *testing.T) {
	store := newStore(t)
	err := store.Import(t.Context(), Import{Reviews: []ReviewedFilm{
		{Name: "Heat", LetterboxdURI: "https://boxd.it/2bfM", ReviewURI: "https://boxd.it/r1",
			ReviewedOn: "2024-01-01", HalfStars: 10, Text: "Great."},
		{Name: "alien", LetterboxdURI: "https://boxd.it/2awY", ReviewURI: "https://boxd.it/r2",
			ReviewedOn: "2024-02-01", Text: "Unrated."},
		{Name: "The Matrix", LetterboxdURI: "https://boxd.it/2a1m", ReviewURI: "https://boxd.it/r3",
			ReviewedOn: "2024-03-01", HalfStars: 6, Text: "Good."},
	}})
	if err != nil {
		t.Fatal(err)
	}

	sorts := map[ReviewSort]string{
		ReviewSortReviewedDesc: "The Matrix alien Heat ",
		ReviewSortReviewedAsc:  "Heat alien The Matrix ",
		// The review without a rating comes last in both directions.
		ReviewSortRatingDesc: "Heat The Matrix alien ",
		ReviewSortRatingAsc:  "The Matrix Heat alien ",
		ReviewSortNameAsc:    "alien Heat The Matrix ",
		ReviewSortNameDesc:   "The Matrix Heat alien ",
	}
	for sort, want := range sorts {
		rec, page := getReviews(t, store, "?sort="+string(sort))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, body = %s", sort, rec.Code, rec.Body)
		} else if got := reviewNames(page); got != want {
			t.Errorf("%s: reviews = %q, want %q", sort, got, want)
		}
	}
	if len(sorts) != len(reviewOrders) {
		t.Errorf("%d sorts tested, there are %d", len(sorts), len(reviewOrders))
	}

	if rec, _ := getReviews(t, store, "?sort=random"); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown sort: status = %d, want 400", rec.Code)
	}
}
