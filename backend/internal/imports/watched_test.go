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
	films, total, err := store.ListWatched(t.Context(), 100, 0)
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
