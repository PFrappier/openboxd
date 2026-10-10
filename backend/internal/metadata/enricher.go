// Package metadata completes the films of the library with the details
// Letterboxd exports don't carry: synopsis, poster, runtime and directors.
//
// It runs in the background, apart from imports, so that an import neither
// waits for TMDB nor fails with it.
package metadata

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"codeberg.org/pfrappier/openboxd/backend/internal/library"
	"codeberg.org/pfrappier/openboxd/backend/internal/tmdb"
)

const (
	batchSize = 100
	// How long to wait after a failed lookup, e.g. when TMDB is unreachable.
	retryDelay = time.Minute
)

// Source is where film details come from: TMDB, or a fake in tests.
type Source interface {
	Find(ctx context.Context, title string, year int) (int64, error)
	Movie(ctx context.Context, id int64) (tmdb.Movie, error)
}

type Enricher struct {
	store      *library.Store
	source     Source
	wake       chan struct{}
	retryDelay time.Duration
}

func NewEnricher(store *library.Store, source Source) *Enricher {
	return &Enricher{
		store:  store,
		source: source,
		// Buffered, so that a Wake during a pass triggers the next one.
		wake:       make(chan struct{}, 1),
		retryDelay: retryDelay,
	}
}

// Wake tells the enricher that films were added to the library.
func (e *Enricher) Wake() {
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

// Run fetches the details of pending films, now and after each Wake, until
// ctx is done.
func (e *Enricher) Run(ctx context.Context) {
	for {
		var retry <-chan time.Time
		if err := e.drain(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.WarnContext(ctx, "fetch film details", "err", err, "retry_in", e.retryDelay)
			retry = time.After(e.retryDelay)
		}

		select {
		case <-ctx.Done():
			return
		case <-e.wake:
		case <-retry:
		}
	}
}

// drain looks up pending films until none is left or a lookup fails.
func (e *Enricher) drain(ctx context.Context) error {
	for {
		films, err := e.store.PendingMetadata(ctx, batchSize)
		if err != nil {
			return err
		}
		if len(films) == 0 {
			return nil
		}
		for _, film := range films {
			if err := e.enrich(ctx, film); err != nil {
				return err
			}
		}
	}
}

func (e *Enricher) enrich(ctx context.Context, film library.PendingFilm) error {
	movie, err := e.lookUp(ctx, film)
	switch {
	case errors.Is(err, tmdb.ErrNotFound):
		return e.store.SetMetadataNotFound(ctx, film.ID)
	case err != nil:
		if ctx.Err() == nil {
			if err := e.store.PostponeMetadata(ctx, film.ID); err != nil {
				return err
			}
		}
		return fmt.Errorf("%q: %w", film.Name, err)
	}

	metadata := library.Metadata{
		TMDBID:     movie.ID,
		Overview:   movie.Overview,
		PosterPath: movie.PosterPath,
		Runtime:    movie.Runtime,
	}
	for _, d := range movie.Directors {
		metadata.Directors = append(metadata.Directors, library.Person{TMDBID: d.ID, Name: d.Name})
	}
	return e.store.SetMetadata(ctx, film.ID, metadata)
}

func (e *Enricher) lookUp(ctx context.Context, film library.PendingFilm) (tmdb.Movie, error) {
	year := 0
	if film.Year != nil {
		year = *film.Year
	}
	id, err := e.source.Find(ctx, film.Name, year)
	if err != nil {
		return tmdb.Movie{}, err
	}
	return e.source.Movie(ctx, id)
}
