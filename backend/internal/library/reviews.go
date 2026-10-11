package library

import (
	"context"
	"fmt"
)

// ReviewedFilm is a review of a film as an export brings it.
type ReviewedFilm struct {
	Name string
	Year *int
	// LetterboxdURI is the one of the film.
	LetterboxdURI string
	// ReviewURI is the one of the review, which identifies it.
	ReviewURI string
	// ReviewedOn is the day the review was written (YYYY-MM-DD).
	ReviewedOn string
	// WatchedOn is the day of the viewing the review is about (YYYY-MM-DD);
	// empty for a review written without logging one.
	WatchedOn string
	// HalfStars goes from 1 (½) to 10 (★★★★★); 0 for a review without rating.
	HalfStars int
	Rewatch   bool
	// Text is as written on Letterboxd, which allows a few HTML tags in it.
	Text string
}

// Review is something the user wrote about a film.
type Review struct {
	ID int64 `json:"id"`
	// ReviewedOn is the day the review was written (YYYY-MM-DD).
	ReviewedOn string `json:"reviewedOn"`
	// WatchedOn is the day of the viewing the review is about (YYYY-MM-DD);
	// null for a review written without logging one.
	WatchedOn *string `json:"watchedOn"`
	// Rating is the one given along with the review, in stars from 0.5 to 5:
	// it can differ from the current rating of the film. Null without one.
	Rating *float64 `json:"rating"`
	// Rewatch tells whether the film had been watched before.
	Rewatch bool `json:"rewatch"`
	// Text is as written on Letterboxd, which allows a few HTML tags in it:
	// it isn't safe to render as HTML.
	Text string `json:"text"`
}

// ReviewEntry is a review along with the film it is about.
type ReviewEntry struct {
	Review
	Film DiaryFilm `json:"film"`
}

// ReviewSort is an order to list the reviews in.
type ReviewSort string

const (
	// The day the review was written.
	ReviewSortReviewedDesc ReviewSort = "reviewed-desc"
	ReviewSortReviewedAsc  ReviewSort = "reviewed-asc"
	// The rating given along with the review.
	ReviewSortRatingDesc ReviewSort = "rating-desc"
	ReviewSortRatingAsc  ReviewSort = "rating-asc"
	ReviewSortNameAsc    ReviewSort = "name-asc"
	ReviewSortNameDesc   ReviewSort = "name-desc"
)

// reviewOrders holds what each sort orders review r about film f by. Reviews
// lacking what they are sorted by come last in both directions; the latest
// reviews first then settle the ties of the other sorts, so that pages never
// overlap.
var reviewOrders = map[ReviewSort]string{
	ReviewSortReviewedDesc: `r.reviewed_on DESC, r.id DESC`,
	ReviewSortReviewedAsc:  `r.reviewed_on, r.id`,
	ReviewSortRatingDesc:   `r.half_stars DESC NULLS LAST`,
	ReviewSortRatingAsc:    `r.half_stars NULLS LAST`,
	ReviewSortNameAsc:      `f.name COLLATE NOCASE`,
	ReviewSortNameDesc:     `f.name COLLATE NOCASE DESC`,
}

// Valid reports whether the sort is one the reviews can be listed in.
func (rs ReviewSort) Valid() bool {
	_, ok := reviewOrders[rs]
	return ok
}

// ListReviews returns a page of the reviews, in the order of the sort, along
// with how many reviews there are in total. The sort must be valid.
func (s *Store) ListReviews(ctx context.Context, sort ReviewSort, limit, offset int) ([]ReviewEntry, int, error) {
	order, ok := reviewOrders[sort]
	if !ok {
		return nil, 0, fmt.Errorf("library: unknown sort %q", sort)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM reviews`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.reviewed_on, r.watched_on, r.half_stars / 2.0, r.rewatch, r.body,
		       f.id, f.name, f.year, f.poster_path, `+liked+`
		FROM reviews r
		JOIN films f ON f.id = r.film_id
		ORDER BY `+order+`, r.reviewed_on DESC, r.id DESC
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// Not nil, so that an empty page encodes as [] rather than null.
	reviews := []ReviewEntry{}
	for rows.Next() {
		var r ReviewEntry
		err := rows.Scan(&r.ID, &r.ReviewedOn, &r.WatchedOn, &r.Rating, &r.Rewatch, &r.Text,
			&r.Film.ID, &r.Film.Name, &r.Film.Year, &r.Film.PosterPath, &r.Film.Liked)
		if err != nil {
			return nil, 0, err
		}
		reviews = append(reviews, r)
	}
	return reviews, total, rows.Err()
}

// reviews returns the reviews of a film, latest first.
func (s *Store) reviews(ctx context.Context, filmID int64) ([]Review, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, reviewed_on, watched_on, half_stars / 2.0, rewatch, body
		FROM reviews
		WHERE film_id = ?
		ORDER BY reviewed_on DESC, id DESC`, filmID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Not nil, so that a film without review encodes them as [].
	reviews := []Review{}
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.ReviewedOn, &r.WatchedOn, &r.Rating, &r.Rewatch, &r.Text); err != nil {
			return nil, err
		}
		reviews = append(reviews, r)
	}
	return reviews, rows.Err()
}

// upsertReview is the statement Import records a ReviewedFilm with; its
// arguments are the film ID, then the review URI, the day it was written, the
// day of the viewing, the half stars, whether it is a rewatch and the text.
const upsertReview = `
	INSERT INTO reviews (film_id, letterboxd_uri, reviewed_on, watched_on, half_stars, rewatch, body)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT (letterboxd_uri) DO UPDATE
	SET film_id = excluded.film_id, reviewed_on = excluded.reviewed_on,
	    watched_on = excluded.watched_on, half_stars = excluded.half_stars,
	    rewatch = excluded.rewatch, body = excluded.body`
