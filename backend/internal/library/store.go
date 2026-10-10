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

	// Rating and Liked are only read from the library: Import takes them from
	// its own Ratings and Likes. Rating is in stars, from 0.5 to 5, and null
	// for a film the user didn't rate.
	Rating *float64 `json:"rating"`
	Liked  bool     `json:"liked"`

	// The details below come from TMDB, not from the export: Import ignores
	// them, and they are null until fetched or when TMDB lacks them.
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
	// Rating is in stars, from 0.5 to 5, and null for a film the user didn't rate.
	Rating *float64 `json:"rating"`
	Liked  bool     `json:"liked"`

	// The details below come from TMDB, as in WatchedFilm.
	TMDBID     *int64   `json:"tmdbId"`
	Overview   *string  `json:"overview"`
	PosterPath *string  `json:"posterPath"`
	Runtime    *int     `json:"runtime"`
	Directors  []string `json:"directors"`
}

// RatedFilm is a film the user rated.
type RatedFilm struct {
	Name          string
	Year          *int
	LetterboxdURI string
	// HalfStars goes from 1 (½) to 10 (★★★★★).
	HalfStars int
	// RatedOn is the day the film was rated (YYYY-MM-DD).
	RatedOn string
}

// LikedFilm is a film the user liked.
type LikedFilm struct {
	Name          string
	Year          *int
	LetterboxdURI string
	// LikedOn is the day the film was liked (YYYY-MM-DD).
	LikedOn string
}

// Import is what an export brings to the library.
type Import struct {
	Watched []WatchedFilm
	Ratings []RatedFilm
	Likes   []LikedFilm
}

// rating and liked select the user's rating of film f in stars, null when it
// isn't rated, and whether they liked it.
const (
	rating = `(SELECT half_stars / 2.0 FROM ratings WHERE film_id = f.id)`
	liked  = `EXISTS (SELECT 1 FROM likes WHERE film_id = f.id)`
)

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

// AddWatched records films as watched, as Import does.
func (s *Store) AddWatched(ctx context.Context, films []WatchedFilm) error {
	return s.Import(ctx, Import{Watched: films})
}

// Import adds the content of an export to the library, all or none. Films
// already known by their Letterboxd URI are updated, so importing the same
// export twice doesn't duplicate anything.
func (s *Store) Import(ctx context.Context, imp Import) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Statements prepared on a transaction are closed along with it.
	upsertFilm, err := tx.PrepareContext(ctx, `
		INSERT INTO films (letterboxd_uri, name, year) VALUES (?, ?, ?)
		ON CONFLICT (letterboxd_uri) DO UPDATE SET name = excluded.name, year = excluded.year
		RETURNING id`)
	if err != nil {
		return err
	}
	filmID := func(uri, name string, year *int) (id int64, err error) {
		err = upsertFilm.QueryRowContext(ctx, uri, name, year).Scan(&id)
		return id, err
	}

	upsertWatched, err := tx.PrepareContext(ctx, `
		INSERT INTO watched (film_id, watched_on) VALUES (?, ?)
		ON CONFLICT (film_id) DO UPDATE SET watched_on = excluded.watched_on`)
	if err != nil {
		return err
	}
	for _, f := range imp.Watched {
		id, err := filmID(f.LetterboxdURI, f.Name, f.Year)
		if err != nil {
			return err
		}
		if _, err := upsertWatched.ExecContext(ctx, id, f.WatchedOn); err != nil {
			return err
		}
	}

	upsertRating, err := tx.PrepareContext(ctx, `
		INSERT INTO ratings (film_id, half_stars, rated_on) VALUES (?, ?, ?)
		ON CONFLICT (film_id) DO UPDATE
		SET half_stars = excluded.half_stars, rated_on = excluded.rated_on`)
	if err != nil {
		return err
	}
	for _, f := range imp.Ratings {
		id, err := filmID(f.LetterboxdURI, f.Name, f.Year)
		if err != nil {
			return err
		}
		if _, err := upsertRating.ExecContext(ctx, id, f.HalfStars, f.RatedOn); err != nil {
			return err
		}
	}

	upsertLike, err := tx.PrepareContext(ctx, `
		INSERT INTO likes (film_id, liked_on) VALUES (?, ?)
		ON CONFLICT (film_id) DO UPDATE SET liked_on = excluded.liked_on`)
	if err != nil {
		return err
	}
	for _, f := range imp.Likes {
		id, err := filmID(f.LetterboxdURI, f.Name, f.Year)
		if err != nil {
			return err
		}
		if _, err := upsertLike.ExecContext(ctx, id, f.LikedOn); err != nil {
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
		       `+rating+`, `+liked+`,
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
			&f.Rating, &f.Liked, &f.Overview, &f.PosterPath, &f.Runtime, &directors)
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
		       `+rating+`, `+liked+`,
		       f.tmdb_id, f.overview, f.poster_path, f.runtime, `+directorNames+`
		FROM films f
		LEFT JOIN watched w ON w.film_id = f.id
		WHERE f.id = ?`, id).
		Scan(&f.ID, &f.Name, &f.Year, &f.LetterboxdURI, &f.WatchedOn,
			&f.Rating, &f.Liked, &f.TMDBID, &f.Overview, &f.PosterPath, &f.Runtime, &directors)
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
