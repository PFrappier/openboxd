package library

import (
	"reflect"
	"testing"
)

func TestSetMetadata(t *testing.T) {
	store := newStore(t)
	film := WatchedFilm{Name: "The Matrix", LetterboxdURI: "https://boxd.it/2a1m", WatchedOn: "2024-01-01"}
	if err := store.AddWatched(t.Context(), []WatchedFilm{film}); err != nil {
		t.Fatal(err)
	}
	pending, err := store.PendingMetadata(t.Context(), 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending = %v, %v, want the imported film", pending, err)
	}
	id := pending[0].ID

	directors := []Person{{TMDBID: 9340, Name: "Lana Wachowski"}, {TMDBID: 9339, Name: "Lilly Wachowski"}}
	// Storing twice, as after a refresh, must not duplicate the directors.
	for range 2 {
		err := store.SetMetadata(t.Context(), id, Metadata{TMDBID: 603, PosterPath: "/matrix.jpg", Runtime: 136, Directors: directors})
		if err != nil {
			t.Fatal(err)
		}
	}
	// Importing the export again must keep what was fetched.
	if err := store.AddWatched(t.Context(), []WatchedFilm{film}); err != nil {
		t.Fatal(err)
	}

	var status, poster string
	var tmdbID, runtime int
	var overview *string
	err = store.db.QueryRow(`SELECT metadata_status, tmdb_id, overview, poster_path, runtime FROM films WHERE id = ?`, id).
		Scan(&status, &tmdbID, &overview, &poster, &runtime)
	if err != nil {
		t.Fatal(err)
	}
	if status != "matched" || tmdbID != 603 || overview != nil || poster != "/matrix.jpg" || runtime != 136 {
		t.Errorf("film = %s, %d, %v, %s, %d", status, tmdbID, overview, poster, runtime)
	}

	rows, err := store.db.Query(`
		SELECT p.tmdb_id, p.name
		FROM film_directors d
		JOIN people p ON p.id = d.person_id
		WHERE d.film_id = ?
		ORDER BY d.position`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []Person
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.TMDBID, &p.Name); err != nil {
			t.Fatal(err)
		}
		got = append(got, p)
	}
	if !reflect.DeepEqual(got, directors) {
		t.Errorf("directors = %v, want %v", got, directors)
	}

	if pending, err := store.PendingMetadata(t.Context(), 10); err != nil || len(pending) != 0 {
		t.Errorf("pending = %v, %v, want none", pending, err)
	}
}
