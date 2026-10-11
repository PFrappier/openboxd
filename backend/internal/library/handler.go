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
// rating, liked and decade parameters narrow down, see watchedFilter, and the
// optional sort parameter orders: a WatchedSort, most recently watched first
// by default.
func (h *Handler) Watched(w http.ResponseWriter, r *http.Request) {
	limit, offset, problem := page(r)
	if problem != "" {
		respond.Error(w, http.StatusBadRequest, problem)
		return
	}

	filter, problem := watchedFilter(r)
	if problem != "" {
		respond.Error(w, http.StatusBadRequest, problem)
		return
	}

	sort := SortAddedDesc
	if raw := r.URL.Query().Get("sort"); raw != "" {
		sort = WatchedSort(raw)
	}
	if !sort.Valid() {
		respond.Error(w, http.StatusBadRequest, "sort is not a known order")
		return
	}

	films, total, err := h.store.ListWatched(r.Context(), filter, sort, limit, offset)
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

// Diary handles GET /api/diary?limit=50&offset=0, which the optional year,
// rating, liked and rewatch parameters narrow down, see diaryFilter, and the
// optional sort parameter orders: a DiarySort, latest viewings first by
// default. Along with the page come the years the whole diary spans.
func (h *Handler) Diary(w http.ResponseWriter, r *http.Request) {
	limit, offset, problem := page(r)
	if problem != "" {
		respond.Error(w, http.StatusBadRequest, problem)
		return
	}

	filter, problem := diaryFilter(r)
	if problem != "" {
		respond.Error(w, http.StatusBadRequest, problem)
		return
	}

	sort := DiarySortWatchedDesc
	if raw := r.URL.Query().Get("sort"); raw != "" {
		sort = DiarySort(raw)
	}
	if !sort.Valid() {
		respond.Error(w, http.StatusBadRequest, "sort is not a known order")
		return
	}

	entries, total, err := h.store.ListDiary(r.Context(), filter, sort, limit, offset)
	if err != nil {
		slog.ErrorContext(r.Context(), "list diary entries", "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not list diary entries")
		return
	}
	years, err := h.store.DiaryYears(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list diary years", "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not list diary entries")
		return
	}
	respond.JSON(w, http.StatusOK, struct {
		Total   int          `json:"total"`
		Years   []int        `json:"years"`
		Entries []DiaryEntry `json:"entries"`
	}{total, years, entries})
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
	var ok bool

	if filter.HalfStars, filter.Unrated, ok = ratingParam(query.Get("rating")); !ok {
		return filter, problemRating
	}
	if filter.Liked, ok = flagParam(query.Get("liked")); !ok {
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

// diaryFilter reads the filter of a request for diary entries:
//   - year: the one of the viewings, e.g. 2024;
//   - rating: a number of stars from 0.5 to 5, or "none" for viewings
//     without a rating;
//   - liked: "true" for liked films only;
//   - rewatch: "true" for films watched before only.
//
// It returns what is wrong with the request, if anything.
func diaryFilter(r *http.Request) (filter DiaryFilter, problem string) {
	query := r.URL.Query()
	var ok bool

	if raw := query.Get("year"); raw != "" {
		year, err := strconv.Atoi(raw)
		if err != nil || year < 1000 || year > 9999 {
			return filter, "year must be a four-digit year"
		}
		filter.Year = &year
	}
	if filter.HalfStars, filter.Unrated, ok = ratingParam(query.Get("rating")); !ok {
		return filter, problemRating
	}
	if filter.Liked, ok = flagParam(query.Get("liked")); !ok {
		return filter, `liked must be "true"`
	}
	if filter.Rewatch, ok = flagParam(query.Get("rewatch")); !ok {
		return filter, `rewatch must be "true"`
	}
	return filter, ""
}

const problemRating = `rating must be "none" or a number of stars from 0.5 to 5`

// ratingParam reads a rating to filter by: a number of stars from 0.5 to 5,
// given back in half stars, or "none" for what isn't rated. An empty one
// filters nothing.
func ratingParam(raw string) (halfStars *int, unrated, ok bool) {
	switch raw {
	case "":
		return nil, false, true
	case "none":
		return nil, true, true
	}
	stars, err := strconv.ParseFloat(raw, 64)
	n := int(stars * 2)
	if err != nil || float64(n) != stars*2 || n < 1 || n > 10 {
		return nil, false, false
	}
	return &n, false, true
}

// flagParam reads a parameter that is either "true" or left out.
func flagParam(raw string) (set, ok bool) {
	return raw == "true", raw == "" || raw == "true"
}

// page reads the limit and offset parameters of a request for a list, the
// limit capped to maxPageSize. It returns what is wrong with the request, if
// anything.
func page(r *http.Request) (limit, offset int, problem string) {
	limit, ok := queryInt(r, "limit", defaultPageSize)
	if !ok || limit < 1 {
		return 0, 0, "limit must be a positive integer"
	}
	offset, ok = queryInt(r, "offset", 0)
	if !ok || offset < 0 {
		return 0, 0, "offset must be a non-negative integer"
	}
	return min(limit, maxPageSize), offset, ""
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
