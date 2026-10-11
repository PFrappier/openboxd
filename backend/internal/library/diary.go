package library

import (
	"context"
	"fmt"
)

// LoggedFilm is a viewing of a film the user logged in their diary.
type LoggedFilm struct {
	Name string
	Year *int
	// LetterboxdURI is the one of the film.
	LetterboxdURI string
	// EntryURI is the one of the diary entry, which identifies the viewing.
	EntryURI string
	// WatchedOn is the day the film was watched (YYYY-MM-DD).
	WatchedOn string
	// HalfStars goes from 1 (½) to 10 (★★★★★); 0 for a viewing without rating.
	HalfStars int
	Rewatch   bool
}

// Viewing is one time the user watched a film.
type Viewing struct {
	ID int64 `json:"id"`
	// WatchedOn is the day the film was watched (YYYY-MM-DD).
	WatchedOn string `json:"watchedOn"`
	// Rating is the one given along with this viewing, in stars from 0.5 to
	// 5: it can differ from the current rating of the film. Null without one.
	Rating *float64 `json:"rating"`
	// Rewatch tells whether the film had been watched before.
	Rewatch bool `json:"rewatch"`
}

// DiaryEntry is a viewing along with the film it is about.
type DiaryEntry struct {
	Viewing
	Film DiaryFilm `json:"film"`
}

// DiaryFilm is what the diary shows of a film.
type DiaryFilm struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Year *int   `json:"year"`
	// PosterPath is relative to TMDB's image CDN, null until fetched or when
	// TMDB has no poster.
	PosterPath *string `json:"posterPath"`
	Liked      bool    `json:"liked"`
}

// DiaryFilter narrows the diary down; its zero value keeps every entry.
type DiaryFilter struct {
	// Year keeps the viewings of that year.
	Year *int
	// HalfStars keeps the viewings rated so, from 1 (½) to 10 (★★★★★).
	HalfStars *int
	// Unrated keeps the viewings logged without a rating.
	Unrated bool
	// Liked keeps the viewings of films the user liked.
	Liked bool
	// Rewatch keeps the viewings of films watched before.
	Rewatch bool
}

// where returns the conditions of the filter on entry e about film f, to put
// after WHERE, and their arguments.
func (df DiaryFilter) where() (string, []any) {
	conditions := "TRUE"
	var args []any
	and := func(condition string, conditionArgs ...any) {
		conditions += " AND " + condition
		args = append(args, conditionArgs...)
	}
	if df.Year != nil {
		// Dates are YYYY-MM-DD: comparing them as texts keeps the index usable.
		and(`e.watched_on BETWEEN ? AND ?`, fmt.Sprintf("%04d-01-01", *df.Year), fmt.Sprintf("%04d-12-31", *df.Year))
	}
	if df.HalfStars != nil {
		and(`e.half_stars = ?`, *df.HalfStars)
	}
	if df.Unrated {
		and(`e.half_stars IS NULL`)
	}
	if df.Liked {
		and(liked)
	}
	if df.Rewatch {
		and(`e.rewatch = 1`)
	}
	return conditions, args
}

// DiarySort is an order to list the diary in.
type DiarySort string

const (
	// The day the film was watched.
	DiarySortWatchedDesc DiarySort = "watched-desc"
	DiarySortWatchedAsc  DiarySort = "watched-asc"
	// The rating given along with the viewing.
	DiarySortRatingDesc DiarySort = "rating-desc"
	DiarySortRatingAsc  DiarySort = "rating-asc"
	DiarySortNameAsc    DiarySort = "name-asc"
	DiarySortNameDesc   DiarySort = "name-desc"
	// The year the film was released.
	DiarySortReleaseDesc DiarySort = "release-desc"
	DiarySortReleaseAsc  DiarySort = "release-asc"
)

// diaryOrders holds what each sort orders entry e about film f by. Entries
// lacking what they are sorted by come last in both directions. Entries of
// the same day follow the order of the export, which lists them as they were
// logged; the latest viewings first then settle the ties of the other sorts,
// so that pages never overlap.
var diaryOrders = map[DiarySort]string{
	DiarySortWatchedDesc: `e.watched_on DESC, e.id DESC`,
	DiarySortWatchedAsc:  `e.watched_on, e.id`,
	DiarySortRatingDesc:  `e.half_stars DESC NULLS LAST`,
	DiarySortRatingAsc:   `e.half_stars NULLS LAST`,
	DiarySortNameAsc:     `f.name COLLATE NOCASE`,
	DiarySortNameDesc:    `f.name COLLATE NOCASE DESC`,
	DiarySortReleaseDesc: `f.year DESC NULLS LAST`,
	DiarySortReleaseAsc:  `f.year NULLS LAST`,
}

// Valid reports whether the sort is one the diary can be listed in.
func (ds DiarySort) Valid() bool {
	_, ok := diaryOrders[ds]
	return ok
}

// ListDiary returns a page of the entries of the diary the filter keeps, in
// the order of the sort, along with how many entries the filter keeps in
// total. The sort must be valid.
func (s *Store) ListDiary(ctx context.Context, filter DiaryFilter, sort DiarySort, limit, offset int) ([]DiaryEntry, int, error) {
	where, args := filter.where()
	order, ok := diaryOrders[sort]
	if !ok {
		return nil, 0, fmt.Errorf("library: unknown sort %q", sort)
	}

	var total int
	err := s.db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM diary_entries e
		JOIN films f ON f.id = e.film_id
		WHERE `+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.watched_on, e.half_stars / 2.0, e.rewatch,
		       f.id, f.name, f.year, f.poster_path, `+liked+`
		FROM diary_entries e
		JOIN films f ON f.id = e.film_id
		WHERE `+where+`
		ORDER BY `+order+`, e.watched_on DESC, e.id DESC
		LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// Not nil, so that an empty page encodes as [] rather than null.
	entries := []DiaryEntry{}
	for rows.Next() {
		var e DiaryEntry
		err := rows.Scan(&e.ID, &e.WatchedOn, &e.Rating, &e.Rewatch,
			&e.Film.ID, &e.Film.Name, &e.Film.Year, &e.Film.PosterPath, &e.Film.Liked)
		if err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}

// DiaryYears returns the years the diary has viewings in, latest first.
func (s *Store) DiaryYears(ctx context.Context) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT CAST(substr(watched_on, 1, 4) AS INTEGER) AS year
		FROM diary_entries
		ORDER BY year DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Not nil, so that an empty diary encodes them as [].
	years := []int{}
	for rows.Next() {
		var year int
		if err := rows.Scan(&year); err != nil {
			return nil, err
		}
		years = append(years, year)
	}
	return years, rows.Err()
}

// viewings returns the diary entries about a film, latest first.
func (s *Store) viewings(ctx context.Context, filmID int64) ([]Viewing, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, watched_on, half_stars / 2.0, rewatch
		FROM diary_entries
		WHERE film_id = ?
		ORDER BY watched_on DESC, id DESC`, filmID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Not nil, so that a film without viewing encodes them as [].
	viewings := []Viewing{}
	for rows.Next() {
		var v Viewing
		if err := rows.Scan(&v.ID, &v.WatchedOn, &v.Rating, &v.Rewatch); err != nil {
			return nil, err
		}
		viewings = append(viewings, v)
	}
	return viewings, rows.Err()
}

// upsertDiaryEntry is the statement Import records a LoggedFilm with; its
// arguments are the film ID, then the entry URI, the day, the half stars and
// whether it is a rewatch.
const upsertDiaryEntry = `
	INSERT INTO diary_entries (film_id, letterboxd_uri, watched_on, half_stars, rewatch)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT (letterboxd_uri) DO UPDATE
	SET film_id = excluded.film_id, watched_on = excluded.watched_on,
	    half_stars = excluded.half_stars, rewatch = excluded.rewatch`
