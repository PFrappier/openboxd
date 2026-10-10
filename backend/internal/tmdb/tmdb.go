// Package tmdb fetches film details from The Movie Database (themoviedb.org).
package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultAPIKey is the key of the Openboxd project, used by instances that
// don't set their own. It only gives read access to public data.
const DefaultAPIKey = "4319fb6c960d9e73fcd48fe5757c0901"

const requestTimeout = 15 * time.Second

// ErrNotFound is returned when TMDB doesn't know the requested film.
var ErrNotFound = errors.New("tmdb: not found")

type Client struct {
	key      string
	language string
	baseURL  string
	http     *http.Client
}

// New returns a client that authenticates with a v3 API key and asks for
// texts in language, a tag such as "fr-FR".
func New(key, language string) *Client {
	return &Client{
		key:      key,
		language: language,
		baseURL:  "https://api.themoviedb.org/3",
		http:     &http.Client{Timeout: requestTimeout},
	}
}

type Person struct {
	ID   int64
	Name string
}

type Movie struct {
	ID int64
	// Overview is empty when TMDB has no synopsis in the client's language.
	Overview string
	// PosterPath is relative to TMDB's image CDN; empty without a poster.
	PosterPath string
	// Runtime is in minutes, 0 when unknown.
	Runtime   int
	Directors []Person
}

// Find returns the ID of the film with the given title, released in year
// (0 when unknown). Among the results it prefers one whose title is exactly
// the given one over TMDB's own ranking.
func (c *Client) Find(ctx context.Context, title string, year int) (int64, error) {
	query := url.Values{"query": {title}}
	if year != 0 {
		query.Set("year", strconv.Itoa(year))
	}
	var page struct {
		Results []struct {
			ID            int64  `json:"id"`
			Title         string `json:"title"`
			OriginalTitle string `json:"original_title"`
		} `json:"results"`
	}
	if err := c.get(ctx, "/search/movie", query, &page); err != nil {
		return 0, err
	}
	if len(page.Results) == 0 {
		return 0, ErrNotFound
	}
	for _, r := range page.Results {
		if strings.EqualFold(r.Title, title) || strings.EqualFold(r.OriginalTitle, title) {
			return r.ID, nil
		}
	}
	return page.Results[0].ID, nil
}

// Movie returns the details of a film, directors included.
func (c *Client) Movie(ctx context.Context, id int64) (Movie, error) {
	query := url.Values{"append_to_response": {"credits"}, "language": {c.language}}
	var body struct {
		Overview   string `json:"overview"`
		PosterPath string `json:"poster_path"`
		Runtime    int    `json:"runtime"`
		Credits    struct {
			Crew []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
				Job  string `json:"job"`
			} `json:"crew"`
		} `json:"credits"`
	}
	if err := c.get(ctx, "/movie/"+strconv.FormatInt(id, 10), query, &body); err != nil {
		return Movie{}, err
	}

	movie := Movie{ID: id, Overview: body.Overview, PosterPath: body.PosterPath, Runtime: body.Runtime}
	for _, member := range body.Credits.Crew {
		if member.Job == "Director" {
			movie.Directors = append(movie.Directors, Person{ID: member.ID, Name: member.Name})
		}
	}
	return movie, nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	query.Set("api_key", c.key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		// A *url.Error spells out the URL, API key included: keep the cause only.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("tmdb: GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("tmdb: GET %s: %s", path, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("tmdb: GET %s: %w", path, err)
	}
	return nil
}
