package library

import (
	"context"
	"fmt"
)

// WatchlistFilm is a film the user wants to watch.
type WatchlistFilm struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Year is null for films without a release year on Letterboxd.
	Year          *int   `json:"year"`
	LetterboxdURI string `json:"letterboxdUri"`
	// AddedOn is the day the film was added to the watchlist (YYYY-MM-DD).
	AddedOn string `json:"addedOn"`

	// The details below come from TMDB, not from the export: Import ignores
	// them, and they are null until fetched or when TMDB lacks them.
	// PosterPath is relative to TMDB's image CDN, e.g. "/8Gxv8gSFCU0.jpg".
	PosterPath *string `json:"posterPath"`
	// Runtime is in minutes.
	Runtime *int `json:"runtime"`
}

// WatchlistFilter narrows the watchlist down; its zero value keeps it all.
type WatchlistFilter struct {
	// Decade keeps the films released in the ten years from it, e.g. 1990.
	Decade *int
}

// where returns the conditions of the filter on film f, to put after WHERE,
// and their arguments.
func (wf WatchlistFilter) where() (string, []any) {
	if wf.Decade != nil {
		return `f.year BETWEEN ? AND ?`, []any{*wf.Decade, *wf.Decade + 9}
	}
	return "TRUE", nil
}

// WatchlistSort is an order to list the watchlist in.
type WatchlistSort string

const (
	// The day the film was added to the watchlist.
	WatchlistSortAddedDesc WatchlistSort = "added-desc"
	WatchlistSortAddedAsc  WatchlistSort = "added-asc"
	WatchlistSortNameAsc   WatchlistSort = "name-asc"
	WatchlistSortNameDesc  WatchlistSort = "name-desc"
	// The year the film was released.
	WatchlistSortReleaseDesc WatchlistSort = "release-desc"
	WatchlistSortReleaseAsc  WatchlistSort = "release-asc"
	WatchlistSortRuntimeDesc WatchlistSort = "runtime-desc"
	WatchlistSortRuntimeAsc  WatchlistSort = "runtime-asc"
)

// watchlistOrders holds what each sort orders film f, in the watchlist as l,
// by. Films lacking what they are sorted by come last in both directions; the
// name then the ID settle the ties, so that pages never overlap.
var watchlistOrders = map[WatchlistSort]string{
	WatchlistSortAddedDesc:   `l.added_on DESC`,
	WatchlistSortAddedAsc:    `l.added_on`,
	WatchlistSortNameAsc:     `f.name COLLATE NOCASE`,
	WatchlistSortNameDesc:    `f.name COLLATE NOCASE DESC`,
	WatchlistSortReleaseDesc: `f.year DESC NULLS LAST`,
	WatchlistSortReleaseAsc:  `f.year NULLS LAST`,
	WatchlistSortRuntimeDesc: `f.runtime DESC NULLS LAST`,
	WatchlistSortRuntimeAsc:  `f.runtime NULLS LAST`,
}

// Valid reports whether the sort is one the watchlist can be listed in.
func (ws WatchlistSort) Valid() bool {
	_, ok := watchlistOrders[ws]
	return ok
}

// ListWatchlist returns a page of the films of the watchlist the filter
// keeps, in the order of the sort, along with how many films the filter keeps
// in total. The sort must be valid.
func (s *Store) ListWatchlist(ctx context.Context, filter WatchlistFilter, sort WatchlistSort, limit, offset int) ([]WatchlistFilm, int, error) {
	where, args := filter.where()
	order, ok := watchlistOrders[sort]
	if !ok {
		return nil, 0, fmt.Errorf("library: unknown sort %q", sort)
	}

	var total int
	err := s.db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM watchlist l
		JOIN films f ON f.id = l.film_id
		WHERE `+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT f.id, f.name, f.year, f.letterboxd_uri, l.added_on, f.poster_path, f.runtime
		FROM watchlist l
		JOIN films f ON f.id = l.film_id
		WHERE `+where+`
		ORDER BY `+order+`, f.name COLLATE NOCASE, f.id
		LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// Not nil, so that an empty page encodes as [] rather than null.
	films := []WatchlistFilm{}
	for rows.Next() {
		var f WatchlistFilm
		err := rows.Scan(&f.ID, &f.Name, &f.Year, &f.LetterboxdURI, &f.AddedOn, &f.PosterPath, &f.Runtime)
		if err != nil {
			return nil, 0, err
		}
		films = append(films, f)
	}
	return films, total, rows.Err()
}

// upsertWatchlistFilm is the statement Import records a WatchlistFilm with;
// its arguments are the film ID, then the day it was added.
const upsertWatchlistFilm = `
	INSERT INTO watchlist (film_id, added_on) VALUES (?, ?)
	ON CONFLICT (film_id) DO UPDATE SET added_on = excluded.added_on`
