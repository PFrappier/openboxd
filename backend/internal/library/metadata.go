package library

import (
	"context"
	"database/sql"
	"time"
)

// PendingFilm is a film whose details haven't been fetched yet.
type PendingFilm struct {
	ID   int64
	Name string
	// Year is nil for films without a release year on Letterboxd.
	Year *int
}

type Person struct {
	TMDBID int64
	Name   string
}

// Metadata holds the details of a film that come from TMDB. Zero values stand
// for details TMDB doesn't have.
type Metadata struct {
	TMDBID     int64
	Overview   string
	PosterPath string
	// Runtime is in minutes.
	Runtime   int
	Directors []Person
}

// PendingMetadata returns up to limit films waiting for their details: the
// ones never looked up first, then the ones postponed the longest ago.
func (s *Store) PendingMetadata(ctx context.Context, limit int) ([]PendingFilm, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, year
		FROM films
		WHERE metadata_status = 'pending'
		ORDER BY metadata_checked_at, id
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var films []PendingFilm
	for rows.Next() {
		var f PendingFilm
		if err := rows.Scan(&f.ID, &f.Name, &f.Year); err != nil {
			return nil, err
		}
		films = append(films, f)
	}
	return films, rows.Err()
}

// SetMetadata stores the details of a film and marks it as matched.
func (s *Store) SetMetadata(ctx context.Context, filmID int64, m Metadata) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		UPDATE films
		SET metadata_status = 'matched', metadata_checked_at = ?,
		    tmdb_id = ?, overview = ?, poster_path = ?, runtime = ?
		WHERE id = ?`,
		now(), m.TMDBID, nullIfZero(m.Overview), nullIfZero(m.PosterPath), nullIfZero(m.Runtime), filmID)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM film_directors WHERE film_id = ?`, filmID); err != nil {
		return err
	}
	for i, d := range m.Directors {
		var personID int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO people (tmdb_id, name) VALUES (?, ?)
			ON CONFLICT (tmdb_id) DO UPDATE SET name = excluded.name
			RETURNING id`, d.TMDBID, d.Name).Scan(&personID)
		if err != nil {
			return err
		}
		// TMDB can credit the same person twice on a film.
		_, err = tx.ExecContext(ctx, `
			INSERT INTO film_directors (film_id, person_id, position) VALUES (?, ?, ?)
			ON CONFLICT DO NOTHING`, filmID, personID, i)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetMetadataNotFound records that TMDB doesn't know the film, so that it
// isn't looked up again.
func (s *Store) SetMetadataNotFound(ctx context.Context, filmID int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE films SET metadata_status = 'not_found', metadata_checked_at = ? WHERE id = ?`,
		now(), filmID)
	return err
}

// PostponeMetadata sends a film whose lookup failed to the back of the
// pending queue, so that it doesn't hold up the others.
func (s *Store) PostponeMetadata(ctx context.Context, filmID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE films SET metadata_checked_at = ? WHERE id = ?`, now(), filmID)
	return err
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func nullIfZero[T comparable](v T) sql.Null[T] {
	var zero T
	return sql.Null[T]{V: v, Valid: v != zero}
}
