package imports

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/pfrappier/openboxd/backend/internal/library"
)

const watchedCSV = "Date,Name,Year,Letterboxd URI\n" +
	"2024-01-01,Alien,1979,https://boxd.it/2awY\n" +
	"2024-03-05,\"Monsters, Inc.\",2001,https://boxd.it/2aGq\n" +
	"2024-02-10,Untitled Project,,https://boxd.it/zzzz\n" +
	"2024-02-11,No URI,2000,\n"

func uploadOK(t *testing.T, h *Handler, files ...upload) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.Upload(rec, newRequest(t, files...))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, body = %s", rec.Code, rec.Body)
	}
}

func listWatched(t *testing.T, store *library.Store) []library.WatchedFilm {
	t.Helper()
	films, total, err := store.ListWatched(t.Context(), library.WatchedFilter{}, library.SortAddedDesc, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != len(films) {
		t.Fatalf("total = %d, got %d films", total, len(films))
	}
	return films
}

func TestUploadImportsWatchedFilms(t *testing.T) {
	uploads := map[string][]upload{
		"zip": {{"export.zip", zipExport(t, "", map[string]string{
			"watched.csv":         watchedCSV,
			"deleted/watched.csv": "Date,Name,Year,Letterboxd URI\n2020-01-01,Gone,1999,https://boxd.it/gone\n",
			"likes/films.csv":     "Date,Name,Year,Letterboxd URI\n2020-01-01,Liked,1999,https://boxd.it/liked\n",
		})}},
		"folder": {{"letterboxd-user/watched.csv", []byte(watchedCSV)}},
	}
	for name, files := range uploads {
		t.Run(name, func(t *testing.T) {
			h, store := newHandler(t, t.TempDir())
			uploadOK(t, h, files...)

			films := listWatched(t, store)
			if len(films) != 3 {
				t.Fatalf("got %d films, want 3: %+v", len(films), films)
			}
			// Most recently watched first.
			first := films[0]
			if first.Name != "Monsters, Inc." || first.WatchedOn != "2024-03-05" ||
				first.LetterboxdURI != "https://boxd.it/2aGq" || first.Year == nil || *first.Year != 2001 {
				t.Errorf("first film = %+v", first)
			}
			if untitled := films[1]; untitled.Name != "Untitled Project" || untitled.Year != nil {
				t.Errorf("film without year = %+v", untitled)
			}
		})
	}
}

func TestUploadTwiceDoesNotDuplicate(t *testing.T) {
	h, store := newHandler(t, t.TempDir())
	uploadOK(t, h, upload{"root/watched.csv", []byte(watchedCSV)})
	uploadOK(t, h, upload{"root/watched.csv", []byte(
		"Date,Name,Year,Letterboxd URI\n2025-06-01,Alien (Director's Cut),1979,https://boxd.it/2awY\n")})

	films := listWatched(t, store)
	if len(films) != 3 {
		t.Fatalf("got %d films, want 3", len(films))
	}
	if films[0].Name != "Alien (Director's Cut)" || films[0].WatchedOn != "2025-06-01" {
		t.Errorf("re-imported film was not updated: %+v", films[0])
	}
}

func TestUploadWithoutWatchedFile(t *testing.T) {
	h, store := newHandler(t, t.TempDir())
	uploadOK(t, h, upload{"root/ratings.csv", []byte("Date,Name,Year,Letterboxd URI,Rating\n")})

	if films := listWatched(t, store); len(films) != 0 {
		t.Errorf("got %d films, want none", len(films))
	}
}

func TestUploadImportsRatingsAndLikes(t *testing.T) {
	const header = "Date,Name,Year,Letterboxd URI"
	h, store := newHandler(t, t.TempDir())
	uploadOK(t, h, upload{"export.zip", zipExport(t, "letterboxd-user/", map[string]string{
		"watched.csv": watchedCSV,
		"ratings.csv": header + ",Rating\n" +
			"2024-01-02,Alien,1979,https://boxd.it/2awY,4.5\n" +
			"2024-03-06,\"Monsters, Inc.\",2001,https://boxd.it/2aGq,0.5\n" +
			// Not ratings Letterboxd can give: left out.
			"2024-02-10,Untitled Project,,https://boxd.it/zzzz,7\n" +
			"2024-02-10,Untitled Project,,https://boxd.it/zzzz,3.2\n" +
			"2024-02-10,Untitled Project,,https://boxd.it/zzzz,\n",
		"likes/films.csv": header + "\n" +
			"2024-01-02,Alien,1979,https://boxd.it/2awY\n" +
			"2020-01-01,Liked,1999,https://boxd.it/liked\n",
		"likes/reviews.csv":   header + "\n2024-01-02,Untitled Project,,https://boxd.it/zzzz\n",
		"deleted/ratings.csv": header + ",Rating\n2020-01-01,Untitled Project,,https://boxd.it/zzzz,5\n",
	})})

	type opinion struct {
		rating float64 // 0 when not rated
		liked  bool
	}
	want := map[string]opinion{
		"Alien":            {4.5, true},
		"Monsters, Inc.":   {0.5, false},
		"Untitled Project": {0, false},
	}
	films := listWatched(t, store)
	// The liked film that isn't in watched.csv doesn't become a watched one.
	if len(films) != len(want) {
		t.Fatalf("got %d films, want %d: %+v", len(films), len(want), films)
	}
	for _, f := range films {
		got := opinion{liked: f.Liked}
		if f.Rating != nil {
			got.rating = *f.Rating
		}
		if got != want[f.Name] {
			t.Errorf("%s: got %+v, want %+v", f.Name, got, want[f.Name])
		}
	}
}

func TestParseRating(t *testing.T) {
	valid := map[string]int{"0.5": 1, "1": 2, "3.5": 7, "5": 10, "5.0": 10}
	for stars, want := range valid {
		if got, ok := parseRating(stars); !ok || got != want {
			t.Errorf("parseRating(%q) = %d, %t, want %d", stars, got, ok, want)
		}
	}
	for _, stars := range []string{"", "0", "5.5", "3.2", "-1", "NaN", "Inf", "four"} {
		if got, ok := parseRating(stars); ok {
			t.Errorf("parseRating(%q) = %d, want it rejected", stars, got)
		}
	}
}

func TestUploadImportsDiary(t *testing.T) {
	const diaryHeader = "Date,Name,Year,Letterboxd URI,Rating,Rewatch,Tags,Watched Date\n"
	h, store := newHandler(t, t.TempDir())
	export := map[string]string{
		"watched.csv": watchedCSV +
			// Two films that only their URI tells apart.
			"2024-04-01,Twins,1990,https://boxd.it/tw1\n" +
			"2024-04-01,Twins,1990,https://boxd.it/tw2\n",
		// The URIs are the ones of the entries, not of the films.
		"diary.csv": diaryHeader +
			"2024-01-05,Alien,1979,https://boxd.it/e1,4.5,,,2024-01-01\n" +
			"2024-06-01,Alien,1979,https://boxd.it/e2,,Yes,\"horror, space\",2024-05-30\n" +
			// Without a watched date, the day it was logged stands for it.
			"2024-02-10,Untitled Project,,https://boxd.it/e3,3,,,\n" +
			// Left out: no film or several ones have this name and year.
			"2024-04-02,Twins,1990,https://boxd.it/e4,2,,,2024-04-01\n" +
			"2024-04-02,Alien,1980,https://boxd.it/e5,2,,,2024-04-01\n" +
			"2024-04-02,Never Watched,2000,https://boxd.it/e6,2,,,2024-04-01\n",
		"deleted/diary.csv": diaryHeader + "2020-01-01,Alien,1979,https://boxd.it/e0,1,,,2020-01-01\n",
	}
	// Importing twice doesn't duplicate the entries.
	for range 2 {
		uploadOK(t, h, upload{"export.zip", zipExport(t, "", export)})
	}

	entries, total, err := store.ListDiary(t.Context(), library.DiaryFilter{}, library.DiarySortWatchedDesc, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	type entry struct {
		film, watchedOn string
		rating          float64 // 0 without rating
		rewatch         bool
	}
	want := []entry{
		{"Alien", "2024-05-30", 0, true},
		{"Untitled Project", "2024-02-10", 3, false},
		{"Alien", "2024-01-01", 4.5, false},
	}
	if total != len(want) || len(entries) != len(want) {
		t.Fatalf("total = %d, %d entries, want %d: %+v", total, len(entries), len(want), entries)
	}
	for i, e := range entries {
		got := entry{film: e.Film.Name, watchedOn: e.WatchedOn, rewatch: e.Rewatch}
		if e.Rating != nil {
			got.rating = *e.Rating
		}
		if got != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got, want[i])
		}
	}
	// Both viewings of Alien are about the film of watched.csv.
	if entries[0].Film.ID != entries[2].Film.ID {
		t.Errorf("the viewings of Alien are about films %d and %d", entries[0].Film.ID, entries[2].Film.ID)
	}
	if films := listWatched(t, store); len(films) != 5 {
		t.Errorf("got %d watched films, want 5: the diary adds none", len(films))
	}
}

func TestUploadImportsWatchlist(t *testing.T) {
	const header = "Date,Name,Year,Letterboxd URI\n"
	h, store := newHandler(t, t.TempDir())
	export := map[string]string{
		"watched.csv": watchedCSV,
		"watchlist.csv": header +
			"2024-05-01,Heat,1995,https://boxd.it/2bfM\n" +
			"2024-06-01,Untitled Sequel,,https://boxd.it/yyyy\n" +
			// Shares its name and year with a watched film, yet isn't it.
			"2024-04-01,Alien,1979,https://boxd.it/other\n" +
			"2024-04-02,No URI,2000,\n",
		"diary.csv": "Date,Name,Year,Letterboxd URI,Rating,Rewatch,Tags,Watched Date\n" +
			"2024-01-05,Alien,1979,https://boxd.it/e1,4.5,,,2024-01-01\n",
		"deleted/watchlist.csv": header + "2020-01-01,Gone,1999,https://boxd.it/gone\n",
	}
	// Importing twice doesn't duplicate the films.
	for range 2 {
		uploadOK(t, h, upload{"export.zip", zipExport(t, "", export)})
	}

	films, total, err := store.ListWatchlist(t.Context(), library.WatchlistFilter{}, library.WatchlistSortAddedDesc, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(films) != 3 {
		t.Fatalf("total = %d, %d films, want 3: %+v", total, len(films), films)
	}
	// Most recently added first.
	if first := films[0]; first.Name != "Untitled Sequel" || first.AddedOn != "2024-06-01" || first.Year != nil {
		t.Errorf("first film = %+v", first)
	}
	if heat := films[1]; heat.Name != "Heat" || heat.LetterboxdURI != "https://boxd.it/2bfM" || heat.Year == nil || *heat.Year != 1995 {
		t.Errorf("second film = %+v", heat)
	}
	if watched := listWatched(t, store); len(watched) != 3 {
		t.Errorf("got %d watched films, want 3: the watchlist adds none", len(watched))
	}

	// The film of the watchlist doesn't blur which Alien the diary is about.
	entries, _, err := store.ListDiary(t.Context(), library.DiaryFilter{}, library.DiarySortWatchedDesc, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Film.ID == films[2].ID {
		t.Errorf("diary entries = %+v, want one about the watched Alien", entries)
	}
}

func TestUploadImportsLists(t *testing.T) {
	h, store := newHandler(t, t.TempDir())
	export := map[string]string{
		"watched.csv": watchedCSV,
		// As Letterboxd writes them: CRLF line ends, a blank line between the tables.
		"lists/top-sci-fi.csv": "Letterboxd list export v7\r\n" +
			"Date,Name,Tags,URL,Description\r\n" +
			"2025-09-03,\"Top, sci-fi\",space,https://boxd.it/top,\"Ranked.\nFor now.\"\r\n" +
			"\r\n" +
			"Position,Name,Year,URL,Description\r\n" +
			"1,Heat,1995,https://boxd.it/2bfM,\r\n" +
			"2,Alien,1979,https://boxd.it/2awY,The first one\r\n" +
			"3,Untitled Sequel,,https://boxd.it/yyyy,\r\n" +
			"4,No URI,2000,,\r\n",
		"lists/empty.csv": "Letterboxd list export v7\nDate,Name,Tags,URL,Description\n" +
			"2025-10-01,Empty,,https://boxd.it/empty,\n\nPosition,Name,Year,URL,Description\n",
		// Can't be told apart from another list without its URI.
		"lists/no-uri.csv":      "Letterboxd list export v7\nDate,Name\n2024-01-01,Nameless\n\nPosition,Name\n1,Alien\n",
		"lists/truncated.csv":   "Letterboxd list export v7\n",
		"deleted/lists/old.csv": "Letterboxd list export v7\nDate,Name,Tags,URL,Description\n2020-01-01,Old,,https://boxd.it/old,\n",
	}
	// Importing twice doesn't duplicate the lists.
	for range 2 {
		uploadOK(t, h, upload{"export.zip", zipExport(t, "", export)})
	}

	lists, total, err := store.ListLists(t.Context(), library.ListSortCreatedDesc, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(lists) != 2 {
		t.Fatalf("total = %d, %d lists, want 2: %+v", total, len(lists), lists)
	}
	if empty := lists[0]; empty.Name != "Empty" || empty.FilmCount != 0 {
		t.Errorf("first list = %+v", empty)
	}
	top := lists[1]
	if top.Name != "Top, sci-fi" || top.LetterboxdURI != "https://boxd.it/top" || top.CreatedOn != "2025-09-03" ||
		top.Description != "Ranked.\nFor now." || top.FilmCount != 3 {
		t.Errorf("second list = %+v", top)
	}

	films, err := store.ListFilms(t.Context(), top.ID, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(films) != 3 {
		t.Fatalf("got %d films, want 3: %+v", len(films), films)
	}
	if heat := films[0]; heat.Name != "Heat" || heat.Position != 1 || heat.Year == nil || *heat.Year != 1995 {
		t.Errorf("first film = %+v", heat)
	}
	if untitled := films[2]; untitled.Name != "Untitled Sequel" || untitled.Position != 3 || untitled.Year != nil {
		t.Errorf("film without year = %+v", untitled)
	}
	// The film of the list is the watched one: they share their URI.
	watched := listWatched(t, store)
	if len(watched) != 3 {
		t.Errorf("got %d watched films, want 3: the lists add none", len(watched))
	}
	if alien := films[1]; alien.ID != watched[2].ID {
		t.Errorf("Alien of the list = %+v, want the watched film %+v", alien, watched[2])
	}
}
