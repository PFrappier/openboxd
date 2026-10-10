// Package library holds the user's films.
package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// ErrNotFound is returned when the library doesn't hold the requested film.
var ErrNotFound = errors.New("library: film not found")

// WatchedFilm is a film the user marked as watched.
type WatchedFilm struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Year is null for films without a release year on Letterboxd.
	Year          *int   `json:"year"`
	LetterboxdURI string `json:"letterboxdUri"`
	// WatchedOn is the day the film was marked as watched (YYYY-MM-DD).
	WatchedOn string `json:"watchedOn"`

	// The details below come from TMDB, not from the export: AddWatched
	// ignores them, and they are null until fetched or when TMDB lacks them.
	Overview *string `json:"overview"`
	// PosterPath is relative to TMDB's image CDN, e.g. "/8Gxv8gSFCU0.jpg".
	PosterPath *string `json:"posterPath"`
	// Runtime is in minutes.
	Runtime *int `json:"runtime"`
	// Directors holds their names, in the order TMDB credits them.
	Directors []string `json:"directors"`
}

// Film is a film of the library, with everything known about it.
type Film struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Year          *int   `json:"year"`
	LetterboxdURI string `json:"letterboxdUri"`
	// WatchedOn is null for a film that isn't marked as watched.
	WatchedOn *string `json:"watchedOn"`

	// The details below come from TMDB, as in WatchedFilm.
	TMDBID     *int64   `json:"tmdbId"`
	Overview   *string  `json:"overview"`
	PosterPath *string  `json:"posterPath"`
	Runtime    *int     `json:"runtime"`
	Directors  []string `json:"directors"`
}

// directorNames selects the names of the directors of film f as a JSON array,
// in the order TMDB credits them; [] for a film without known director.
const directorNames = `(
	SELECT json_group_array(p.name ORDER BY d.position)
	FROM film_directors d
	JOIN people p ON p.id = d.person_id
	WHERE d.film_id = f.id)`

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// AddWatched records films as watched, all or none. Films already known by
// their Letterboxd URI are updated, so importing the same export twice
// doesn't duplicate anything.
func (s *Store) AddWatched(ctx context.Context, films []WatchedFilm) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	upsertFilm, err := tx.PrepareContext(ctx, `
		INSERT INTO films (letterboxd_uri, name, year) VALUES (?, ?, ?)
		ON CONFLICT (letterboxd_uri) DO UPDATE SET name = excluded.name, year = excluded.year
		RETURNING id`)
	if err != nil {
		return err
	}
	defer upsertFilm.Close()

	upsertWatched, err := tx.PrepareContext(ctx, `
		INSERT INTO watched (film_id, watched_on) VALUES (?, ?)
		ON CONFLICT (film_id) DO UPDATE SET watched_on = excluded.watched_on`)
	if err != nil {
		return err
	}
	defer upsertWatched.Close()

	for _, f := range films {
		var id int64
		if err := upsertFilm.QueryRowContext(ctx, f.LetterboxdURI, f.Name, f.Year).Scan(&id); err != nil {
			return err
		}
		if _, err := upsertWatched.ExecContext(ctx, id, f.WatchedOn); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListWatched returns a page of watched films, most recently watched first,
// along with the total number of watched films.
func (s *Store) ListWatched(ctx context.Context, limit, offset int) ([]WatchedFilm, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM watched`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT f.id, f.name, f.year, f.letterboxd_uri, w.watched_on,
		       f.overview, f.poster_path, f.runtime, `+directorNames+`
		FROM watched w
		JOIN films f ON f.id = w.film_id
		ORDER BY w.watched_on DESC, f.name COLLATE NOCASE, f.id
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// Not nil, so that an empty page encodes as [] rather than null.
	films := []WatchedFilm{}
	for rows.Next() {
		var f WatchedFilm
		var directors string
		err := rows.Scan(&f.ID, &f.Name, &f.Year, &f.LetterboxdURI, &f.WatchedOn,
			&f.Overview, &f.PosterPath, &f.Runtime, &directors)
		if err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal([]byte(directors), &f.Directors); err != nil {
			return nil, 0, err
		}
		films = append(films, f)
	}
	return films, total, rows.Err()
}

// Film returns the film with the given ID, or ErrNotFound.
func (s *Store) Film(ctx context.Context, id int64) (Film, error) {
	var f Film
	var directors string
	err := s.db.QueryRowContext(ctx, `
		SELECT f.id, f.name, f.year, f.letterboxd_uri, w.watched_on,
		       f.tmdb_id, f.overview, f.poster_path, f.runtime, `+directorNames+`
		FROM films f
		LEFT JOIN watched w ON w.film_id = f.id
		WHERE f.id = ?`, id).
		Scan(&f.ID, &f.Name, &f.Year, &f.LetterboxdURI, &f.WatchedOn,
			&f.TMDBID, &f.Overview, &f.PosterPath, &f.Runtime, &directors)
	if errors.Is(err, sql.ErrNoRows) {
		return Film{}, ErrNotFound
	}
	if err != nil {
		return Film{}, err
	}
	if err := json.Unmarshal([]byte(directors), &f.Directors); err != nil {
		return Film{}, err
	}
	return f, nil
}
