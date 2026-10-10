package metadata

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"codeberg.org/pfrappier/openboxd/backend/internal/database"
	"codeberg.org/pfrappier/openboxd/backend/internal/library"
	"codeberg.org/pfrappier/openboxd/backend/internal/tmdb"
)

// fakeSource knows the films of ids, by title, and fails on the titles of down.
type fakeSource struct {
	mu    sync.Mutex
	ids   map[string]int64
	down  map[string]bool
	finds []string
}

func (s *fakeSource) Find(_ context.Context, title string, _ int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finds = append(s.finds, title)
	if s.down[title] {
		return 0, errors.New("tmdb is down")
	}
	id, ok := s.ids[title]
	if !ok {
		return 0, tmdb.ErrNotFound
	}
	return id, nil
}

func (s *fakeSource) Movie(_ context.Context, id int64) (tmdb.Movie, error) {
	return tmdb.Movie{ID: id, Overview: "An overview."}, nil
}

func (s *fakeSource) findCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.finds)
}

// recover makes every lookup work again.
func (s *fakeSource) recover() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.down = nil
}

func newStore(t *testing.T, names ...string) *library.Store {
	t.Helper()
	db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	store := library.NewStore(db)
	var films []library.WatchedFilm
	for _, name := range names {
		films = append(films, library.WatchedFilm{
			Name:          name,
			LetterboxdURI: "https://boxd.it/" + name,
			WatchedOn:     "2024-01-01",
		})
	}
	if err := store.AddWatched(t.Context(), films); err != nil {
		t.Fatal(err)
	}
	return store
}

func pendingNames(t *testing.T, store *library.Store) []string {
	t.Helper()
	films, err := store.PendingMetadata(t.Context(), 10)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range films {
		names = append(names, f.Name)
	}
	return names
}

func TestDrainSettlesFoundAndUnknownFilms(t *testing.T) {
	store := newStore(t, "Known", "Unknown")
	source := &fakeSource{ids: map[string]int64{"Known": 1}}

	if err := NewEnricher(store, source).drain(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := pendingNames(t, store); len(got) != 0 {
		t.Errorf("still pending: %v", got)
	}
	if len(source.finds) != 2 {
		t.Errorf("looked up %v, want each film once", source.finds)
	}
}

func TestDrainPostponesFailedLookup(t *testing.T) {
	store := newStore(t, "Broken", "Known")
	source := &fakeSource{ids: map[string]int64{"Known": 1}, down: map[string]bool{"Broken": true}}
	enricher := NewEnricher(store, source)

	if err := enricher.drain(t.Context()); err == nil {
		t.Fatal("drain succeeded although a lookup failed")
	}
	if got := pendingNames(t, store); len(got) != 2 || got[0] != "Known" {
		t.Errorf("pending = %v, want the failed film behind the other", got)
	}

	// The next pass reaches the other film before failing again.
	if err := enricher.drain(t.Context()); err == nil {
		t.Fatal("drain succeeded although a lookup failed")
	}
	if got := pendingNames(t, store); len(got) != 1 || got[0] != "Broken" {
		t.Errorf("pending = %v, want only the failed film", got)
	}
}

func TestRunRetriesAndHandlesWake(t *testing.T) {
	store := newStore(t, "First")
	source := &fakeSource{ids: map[string]int64{"First": 1, "Second": 2}, down: map[string]bool{"First": true}}
	enricher := NewEnricher(store, source)
	enricher.retryDelay = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		enricher.Run(ctx)
	}()
	defer func() {
		cancel()
		<-done
	}()

	waitFor(t, "a first failed lookup", func() bool { return source.findCount() > 0 })
	source.recover()
	waitFor(t, "the retry", func() bool { return len(pendingNames(t, store)) == 0 })

	err := store.AddWatched(ctx, []library.WatchedFilm{
		{Name: "Second", LetterboxdURI: "https://boxd.it/Second", WatchedOn: "2024-01-02"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := pendingNames(t, store); len(got) != 1 {
		t.Fatalf("pending = %v, want the new film to wait for Wake", got)
	}
	enricher.Wake()
	waitFor(t, "the pass after Wake", func() bool { return len(pendingNames(t, store)) == 0 })
}

func waitFor(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
