package tmdb

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func newClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New("secret-key", "fr-FR")
	c.baseURL = srv.URL
	return c
}

func TestFindPrefersExactTitle(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/search/movie" || q.Get("api_key") != "secret-key" || q.Get("query") != "Solaris" || q.Get("year") != "1972" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		w.Write([]byte(`{"results": [
			{"id": 1, "title": "Solaris Rising", "original_title": "Solaris Rising"},
			{"id": 2, "title": "Solaris", "original_title": "Солярис"}
		]}`))
	})

	id, err := c.Find(t.Context(), "Solaris", 1972)
	if err != nil || id != 2 {
		t.Errorf("Find = %d, %v, want 2", id, err)
	}
}

func TestFindFallsBackToFirstResult(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("year") {
			t.Errorf("year sent for a film without one: %s", r.URL)
		}
		w.Write([]byte(`{"results": [{"id": 7, "title": "Amélie"}, {"id": 8, "title": "Amelia"}]}`))
	})

	id, err := c.Find(t.Context(), "Amelie", 0)
	if err != nil || id != 7 {
		t.Errorf("Find = %d, %v, want 7", id, err)
	}
}

func TestFindWithoutResult(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results": []}`))
	})

	if _, err := c.Find(t.Context(), "Nope", 2020); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestMovieKeepsDirectorsOnly(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/movie/2" || q.Get("language") != "fr-FR" || q.Get("append_to_response") != "credits" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		w.Write([]byte(`{
			"overview": "Un psychologue rejoint une station spatiale.",
			"poster_path": "/solaris.jpg",
			"runtime": 167,
			"credits": {"crew": [
				{"id": 10, "name": "Andrei Tarkovsky", "job": "Director"},
				{"id": 11, "name": "Vadim Yusov", "job": "Director of Photography"}
			]}
		}`))
	})

	got, err := c.Movie(t.Context(), 2)
	if err != nil {
		t.Fatal(err)
	}
	want := Movie{
		ID:         2,
		Overview:   "Un psychologue rejoint une station spatiale.",
		PosterPath: "/solaris.jpg",
		Runtime:    167,
		Directors:  []Person{{ID: 10, Name: "Andrei Tarkovsky"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Movie = %+v, want %+v", got, want)
	}
}

func TestMovieStatuses(t *testing.T) {
	status := http.StatusNotFound
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	})

	if _, err := c.Movie(t.Context(), 2); !errors.Is(err, ErrNotFound) {
		t.Errorf("404: err = %v, want ErrNotFound", err)
	}
	status = http.StatusUnauthorized
	if _, err := c.Movie(t.Context(), 2); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("401: err = %v, want an error other than ErrNotFound", err)
	}
}

func TestErrorsDontLeakTheKey(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	c := New("secret-key", "fr-FR")
	c.baseURL = srv.URL
	srv.Close()

	_, err := c.Find(t.Context(), "Solaris", 1972)
	if err == nil || strings.Contains(err.Error(), "secret-key") {
		t.Errorf("err = %v, want a network error without the API key", err)
	}
}
