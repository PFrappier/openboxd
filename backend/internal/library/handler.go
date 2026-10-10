package library

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"codeberg.org/pfrappier/openboxd/backend/internal/respond"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// Watched handles GET /api/watched?limit=50&offset=0, which the optional
// rating, liked and decade parameters narrow down, see watchedFilter.
func (h *Handler) Watched(w http.ResponseWriter, r *http.Request) {
	limit, ok := queryInt(r, "limit", defaultPageSize)
	if !ok || limit < 1 {
		respond.Error(w, http.StatusBadRequest, "limit must be a positive integer")
		return
	}
	offset, ok := queryInt(r, "offset", 0)
	if !ok || offset < 0 {
		respond.Error(w, http.StatusBadRequest, "offset must be a non-negative integer")
		return
	}

	filter, problem := watchedFilter(r)
	if problem != "" {
		respond.Error(w, http.StatusBadRequest, problem)
		return
	}

	films, total, err := h.store.ListWatched(r.Context(), filter, min(limit, maxPageSize), offset)
	if err != nil {
		slog.ErrorContext(r.Context(), "list watched films", "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not list watched films")
		return
	}
	respond.JSON(w, http.StatusOK, struct {
		Total int           `json:"total"`
		Films []WatchedFilm `json:"films"`
	}{total, films})
}

// Film handles GET /api/films/{id}.
func (h *Handler) Film(w http.ResponseWriter, r *http.Request) {
	// An ID that isn't a number can't be the one of a film.
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		respond.Error(w, http.StatusNotFound, "film not found")
		return
	}

	film, err := h.store.Film(r.Context(), id)
	switch {
	case errors.Is(err, ErrNotFound):
		respond.Error(w, http.StatusNotFound, "film not found")
	case err != nil:
		slog.ErrorContext(r.Context(), "get film", "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not get the film")
	default:
		respond.JSON(w, http.StatusOK, film)
	}
}

// watchedFilter reads the filter of a request for watched films:
//   - rating: a number of stars from 0.5 to 5, or "none" for unrated films;
//   - liked: "true" for liked films only;
//   - decade: its first year, e.g. 1990.
//
// It returns what is wrong with the request, if anything.
func watchedFilter(r *http.Request) (filter WatchedFilter, problem string) {
	query := r.URL.Query()

	switch raw := query.Get("rating"); raw {
	case "":
	case "none":
		filter.Unrated = true
	default:
		stars, err := strconv.ParseFloat(raw, 64)
		halfStars := int(stars * 2)
		if err != nil || float64(halfStars) != stars*2 || halfStars < 1 || halfStars > 10 {
			return filter, `rating must be "none" or a number of stars from 0.5 to 5`
		}
		filter.HalfStars = &halfStars
	}

	switch raw := query.Get("liked"); raw {
	case "":
	case "true":
		filter.Liked = true
	default:
		return filter, `liked must be "true"`
	}

	if raw := query.Get("decade"); raw != "" {
		decade, err := strconv.Atoi(raw)
		if err != nil || decade%10 != 0 {
			return filter, "decade must be the first year of a decade"
		}
		filter.Decade = &decade
	}
	return filter, ""
}

// queryInt reads an integer query parameter, or fallback when it is absent.
func queryInt(r *http.Request, name string, fallback int) (int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, true
	}
	n, err := strconv.Atoi(raw)
	return n, err == nil
}
