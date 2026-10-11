// Package library holds the user's films.
package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	// Reviewed tells whether the user wrote a review of the film; as Rating
	// and Liked, it is only read from the library.
	Reviewed bool `json:"reviewed"`

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
	// WatchlistedOn is the day the film was added to the watchlist
	// (YYYY-MM-DD), null for a film that isn't in it.
	WatchlistedOn *string `json:"watchlistedOn"`

	// The details below come from TMDB, as in WatchedFilm.
	TMDBID     *int64   `json:"tmdbId"`
	Overview   *string  `json:"overview"`
	PosterPath *string  `json:"posterPath"`
	Runtime    *int     `json:"runtime"`
	Directors  []string `json:"directors"`

	// Diary holds the viewings the user logged, latest first.
	Diary []Viewing `json:"diary"`
	// Reviews holds what the user wrote about the film, latest first.
	Reviews []Review `json:"reviews"`
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
	Diary   []LoggedFilm
	Reviews []ReviewedFilm
	// Watchlist is added to the one of the library: a film that left the
	// watchlist since an earlier import stays in it.
	Watchlist []WatchlistFilm
	// Lists replace the ones the library knows by their URI, films included;
	// a list deleted since an earlier import stays in the library.
	Lists []ImportedList
}

// rating, liked and reviewed select the user's rating of film f in stars, null
// when it isn't rated, whether they liked it and whether they reviewed it.
const (
	rating   = `(SELECT half_stars / 2.0 FROM ratings WHERE film_id = f.id)`
	liked    = `EXISTS (SELECT 1 FROM likes WHERE film_id = f.id)`
	reviewed = `EXISTS (SELECT 1 FROM reviews WHERE film_id = f.id)`
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

	upsertEntry, err := tx.PrepareContext(ctx, upsertDiaryEntry)
	if err != nil {
		return err
	}
	for _, f := range imp.Diary {
		id, err := filmID(f.LetterboxdURI, f.Name, f.Year)
		if err != nil {
			return err
		}
		_, err = upsertEntry.ExecContext(ctx, id, f.EntryURI, f.WatchedOn, nullIfZero(f.HalfStars), f.Rewatch)
		if err != nil {
			return err
		}
	}

	upsertReviewStmt, err := tx.PrepareContext(ctx, upsertReview)
	if err != nil {
		return err
	}
	for _, f := range imp.Reviews {
		id, err := filmID(f.LetterboxdURI, f.Name, f.Year)
		if err != nil {
			return err
		}
		_, err = upsertReviewStmt.ExecContext(ctx, id, f.ReviewURI, f.ReviewedOn, nullIfZero(f.WatchedOn),
			nullIfZero(f.HalfStars), f.Rewatch, f.Text)
		if err != nil {
			return err
		}
	}

	upsertWatchlist, err := tx.PrepareContext(ctx, upsertWatchlistFilm)
	if err != nil {
		return err
	}
	for _, f := range imp.Watchlist {
		id, err := filmID(f.LetterboxdURI, f.Name, f.Year)
		if err != nil {
			return err
		}
		if _, err := upsertWatchlist.ExecContext(ctx, id, f.AddedOn); err != nil {
			return err
		}
	}

	upsertListStmt, err := tx.PrepareContext(ctx, upsertList)
	if err != nil {
		return err
	}
	clearListStmt, err := tx.PrepareContext(ctx, clearList)
	if err != nil {
		return err
	}
	insertListFilmStmt, err := tx.PrepareContext(ctx, insertListFilm)
	if err != nil {
		return err
	}
	for _, l := range imp.Lists {
		var listID int64
		err := upsertListStmt.QueryRowContext(ctx, l.LetterboxdURI, l.Name, l.Description, l.CreatedOn).Scan(&listID)
		if err != nil {
			return err
		}
		if _, err := clearListStmt.ExecContext(ctx, listID); err != nil {
			return err
		}
		for i, f := range l.Films {
			id, err := filmID(f.LetterboxdURI, f.Name, f.Year)
			if err != nil {
				return err
			}
			if _, err := insertListFilmStmt.ExecContext(ctx, listID, id, i+1); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// WatchedFilter narrows the watched films down; its zero value keeps them all.
type WatchedFilter struct {
	// HalfStars keeps the films rated so, from 1 (½) to 10 (★★★★★).
	HalfStars *int
	// Unrated keeps the films the user didn't rate.
	Unrated bool
	// Liked keeps the films the user liked.
	Liked bool
	// Decade keeps the films released in the ten years from it, e.g. 1990.
	Decade *int
}

// where returns the conditions of the filter on film f, to put after WHERE,
// and their arguments.
func (wf WatchedFilter) where() (string, []any) {
	conditions := "TRUE"
	var args []any
	and := func(condition string, conditionArgs ...any) {
		conditions += " AND " + condition
		args = append(args, conditionArgs...)
	}
	if wf.HalfStars != nil {
		and(`EXISTS (SELECT 1 FROM ratings WHERE film_id = f.id AND half_stars = ?)`, *wf.HalfStars)
	}
	if wf.Unrated {
		and(`NOT EXISTS (SELECT 1 FROM ratings WHERE film_id = f.id)`)
	}
	if wf.Liked {
		and(liked)
	}
	if wf.Decade != nil {
		and(`f.year BETWEEN ? AND ?`, *wf.Decade, *wf.Decade+9)
	}
	return conditions, args
}

// WatchedSort is an order to list the watched films in.
type WatchedSort string

const (
	// The day the film was marked as watched.
	SortAddedDesc WatchedSort = "added-desc"
	SortAddedAsc  WatchedSort = "added-asc"
	SortNameAsc   WatchedSort = "name-asc"
	SortNameDesc  WatchedSort = "name-desc"
	// The year the film was released.
	SortReleaseDesc WatchedSort = "release-desc"
	SortReleaseAsc  WatchedSort = "release-asc"
	// The user's rating of the film.
	SortRatingDesc WatchedSort = "rating-desc"
	SortRatingAsc  WatchedSort = "rating-asc"
	// The day the user rated the film.
	SortRatedDesc   WatchedSort = "rated-desc"
	SortRatedAsc    WatchedSort = "rated-asc"
	SortRuntimeDesc WatchedSort = "runtime-desc"
	SortRuntimeAsc  WatchedSort = "runtime-asc"
)

// watchedOrders holds what each sort orders film f, watched as w, by. Films
// lacking what they are sorted by come last in both directions; the name then
// the ID settle the ties, so that pages never overlap.
var watchedOrders = map[WatchedSort]string{
	SortAddedDesc:   `w.watched_on DESC`,
	SortAddedAsc:    `w.watched_on`,
	SortNameAsc:     `f.name COLLATE NOCASE`,
	SortNameDesc:    `f.name COLLATE NOCASE DESC`,
	SortReleaseDesc: `f.year DESC NULLS LAST`,
	SortReleaseAsc:  `f.year NULLS LAST`,
	SortRatingDesc:  `(SELECT half_stars FROM ratings WHERE film_id = f.id) DESC NULLS LAST`,
	SortRatingAsc:   `(SELECT half_stars FROM ratings WHERE film_id = f.id) NULLS LAST`,
	SortRatedDesc:   `(SELECT rated_on FROM ratings WHERE film_id = f.id) DESC NULLS LAST`,
	SortRatedAsc:    `(SELECT rated_on FROM ratings WHERE film_id = f.id) NULLS LAST`,
	SortRuntimeDesc: `f.runtime DESC NULLS LAST`,
	SortRuntimeAsc:  `f.runtime NULLS LAST`,
}

// Valid reports whether the sort is one the films can be listed in.
func (ws WatchedSort) Valid() bool {
	_, ok := watchedOrders[ws]
	return ok
}

// ListWatched returns a page of the watched films the filter keeps, in the
// order of the sort, along with how many films the filter keeps in total. The
// sort must be valid.
func (s *Store) ListWatched(ctx context.Context, filter WatchedFilter, sort WatchedSort, limit, offset int) ([]WatchedFilm, int, error) {
	where, args := filter.where()
	order, ok := watchedOrders[sort]
	if !ok {
		return nil, 0, fmt.Errorf("library: unknown sort %q", sort)
	}

	var total int
	err := s.db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM watched w
		JOIN films f ON f.id = w.film_id
		WHERE `+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT f.id, f.name, f.year, f.letterboxd_uri, w.watched_on,
		       `+rating+`, `+liked+`, `+reviewed+`,
		       f.overview, f.poster_path, f.runtime, `+directorNames+`
		FROM watched w
		JOIN films f ON f.id = w.film_id
		WHERE `+where+`
		ORDER BY `+order+`, f.name COLLATE NOCASE, f.id
		LIMIT ? OFFSET ?`, append(args, limit, offset)...)
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
			&f.Rating, &f.Liked, &f.Reviewed, &f.Overview, &f.PosterPath, &f.Runtime, &directors)
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
		       `+rating+`, `+liked+`, (SELECT added_on FROM watchlist WHERE film_id = f.id),
		       f.tmdb_id, f.overview, f.poster_path, f.runtime, `+directorNames+`
		FROM films f
		LEFT JOIN watched w ON w.film_id = f.id
		WHERE f.id = ?`, id).
		Scan(&f.ID, &f.Name, &f.Year, &f.LetterboxdURI, &f.WatchedOn,
			&f.Rating, &f.Liked, &f.WatchlistedOn, &f.TMDBID, &f.Overview, &f.PosterPath, &f.Runtime, &directors)
	if errors.Is(err, sql.ErrNoRows) {
		return Film{}, ErrNotFound
	}
	if err != nil {
		return Film{}, err
	}
	if err := json.Unmarshal([]byte(directors), &f.Directors); err != nil {
		return Film{}, err
	}
	if f.Diary, err = s.viewings(ctx, id); err != nil {
		return Film{}, err
	}
	if f.Reviews, err = s.reviews(ctx, id); err != nil {
		return Film{}, err
	}
	return f, nil
}
