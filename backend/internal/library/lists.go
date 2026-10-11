package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrListNotFound is returned when the library doesn't hold the requested list.
var ErrListNotFound = errors.New("library: list not found")

// listPosters is how many films a list shows the posters of as a preview.
const listPosters = 5

// List is a list of films the user made.
type List struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	LetterboxdURI string `json:"letterboxdUri"`
	// CreatedOn is the day the list was made (YYYY-MM-DD).
	CreatedOn string `json:"createdOn"`
	// Description is empty for a list without one.
	Description string `json:"description"`

	// The details below are only read from the library: Import takes the
	// films of a list from ImportedList.
	FilmCount int `json:"filmCount"`
	// Posters holds the poster paths of the first films of the list, relative
	// to TMDB's image CDN; null for a film without a poster, or not yet.
	Posters []*string `json:"posters"`
}

// ImportedList is a list as an export brings it, along with its films.
type ImportedList struct {
	Name          string
	LetterboxdURI string
	CreatedOn     string
	Description   string
	// Films are in the order of the list; only their Name, Year and
	// LetterboxdURI are read.
	Films []ListFilm
}

// ListFilm is a film of a list.
type ListFilm struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Year is null for films without a release year on Letterboxd.
	Year          *int   `json:"year"`
	LetterboxdURI string `json:"letterboxdUri"`
	// Position is the rank of the film in the list, from 1.
	Position int `json:"position"`
	// Rating is in stars, from 0.5 to 5, and null for a film the user didn't rate.
	Rating *float64 `json:"rating"`
	Liked  bool     `json:"liked"`
	// PosterPath is relative to TMDB's image CDN, null until fetched or when
	// TMDB has no poster.
	PosterPath *string `json:"posterPath"`
}

// ListSort is an order to list the lists in.
type ListSort string

const (
	// The day the list was made.
	ListSortCreatedDesc ListSort = "created-desc"
	ListSortCreatedAsc  ListSort = "created-asc"
	ListSortNameAsc     ListSort = "name-asc"
	ListSortNameDesc    ListSort = "name-desc"
	// The number of films of the list.
	ListSortFilmsDesc ListSort = "films-desc"
	ListSortFilmsAsc  ListSort = "films-asc"
)

// listOrders holds what each sort orders list l by; the name then the ID
// settle the ties, so that pages never overlap.
var listOrders = map[ListSort]string{
	ListSortCreatedDesc: `l.created_on DESC`,
	ListSortCreatedAsc:  `l.created_on`,
	ListSortNameAsc:     `l.name COLLATE NOCASE`,
	ListSortNameDesc:    `l.name COLLATE NOCASE DESC`,
	ListSortFilmsDesc:   `film_count DESC`,
	ListSortFilmsAsc:    `film_count`,
}

// Valid reports whether the sort is one the lists can be listed in.
func (ls ListSort) Valid() bool {
	_, ok := listOrders[ls]
	return ok
}

// selectLists selects lists as l, with the columns scanList reads.
const selectLists = `
	SELECT l.id, l.name, l.letterboxd_uri, l.created_on, l.description,
	       (SELECT count(*) FROM list_films WHERE list_id = l.id) AS film_count,
	       (SELECT json_group_array(poster_path ORDER BY position)
	        FROM (SELECT f.poster_path, lf.position
	              FROM list_films lf
	              JOIN films f ON f.id = lf.film_id
	              WHERE lf.list_id = l.id
	              ORDER BY lf.position
	              LIMIT ?))
	FROM lists l`

func scanList(row interface{ Scan(...any) error }) (List, error) {
	var l List
	var posters string
	err := row.Scan(&l.ID, &l.Name, &l.LetterboxdURI, &l.CreatedOn, &l.Description, &l.FilmCount, &posters)
	if err != nil {
		return List{}, err
	}
	return l, json.Unmarshal([]byte(posters), &l.Posters)
}

// ListLists returns a page of the lists, in the order of the sort, along with
// how many lists there are in total. The sort must be valid.
func (s *Store) ListLists(ctx context.Context, sort ListSort, limit, offset int) ([]List, int, error) {
	order, ok := listOrders[sort]
	if !ok {
		return nil, 0, fmt.Errorf("library: unknown sort %q", sort)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM lists`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, selectLists+`
		ORDER BY `+order+`, l.name COLLATE NOCASE, l.id
		LIMIT ? OFFSET ?`, listPosters, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// Not nil, so that an empty page encodes as [] rather than null.
	lists := []List{}
	for rows.Next() {
		l, err := scanList(rows)
		if err != nil {
			return nil, 0, err
		}
		lists = append(lists, l)
	}
	return lists, total, rows.Err()
}

// List returns the list with the given ID, or ErrListNotFound.
func (s *Store) List(ctx context.Context, id int64) (List, error) {
	l, err := scanList(s.db.QueryRowContext(ctx, selectLists+` WHERE l.id = ?`, listPosters, id))
	if errors.Is(err, sql.ErrNoRows) {
		return List{}, ErrListNotFound
	}
	return l, err
}

// ListFilms returns a page of the films of a list, in its order. A list the
// library doesn't hold has none.
func (s *Store) ListFilms(ctx context.Context, listID int64, limit, offset int) ([]ListFilm, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT f.id, f.name, f.year, f.letterboxd_uri, lf.position,
		       `+rating+`, `+liked+`, f.poster_path
		FROM list_films lf
		JOIN films f ON f.id = lf.film_id
		WHERE lf.list_id = ?
		ORDER BY lf.position
		LIMIT ? OFFSET ?`, listID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Not nil, so that an empty page encodes as [] rather than null.
	films := []ListFilm{}
	for rows.Next() {
		var f ListFilm
		err := rows.Scan(&f.ID, &f.Name, &f.Year, &f.LetterboxdURI, &f.Position,
			&f.Rating, &f.Liked, &f.PosterPath)
		if err != nil {
			return nil, err
		}
		films = append(films, f)
	}
	return films, rows.Err()
}

// The statements Import records an ImportedList with. The arguments of
// upsertList are the URI of the list, then its name, its description and the
// day it was made; it returns the ID of the list. Those of insertListFilm are
// the list ID, then the film ID and its position: a film listed twice keeps
// its first one.
const (
	upsertList = `
		INSERT INTO lists (letterboxd_uri, name, description, created_on) VALUES (?, ?, ?, ?)
		ON CONFLICT (letterboxd_uri) DO UPDATE
		SET name = excluded.name, description = excluded.description, created_on = excluded.created_on
		RETURNING id`
	clearList      = `DELETE FROM list_films WHERE list_id = ?`
	insertListFilm = `
		INSERT INTO list_films (list_id, film_id, position) VALUES (?, ?, ?)
		ON CONFLICT (list_id, film_id) DO NOTHING`
)
