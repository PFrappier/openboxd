-- A film is identified by its Letterboxd URI, which every export file that
-- lists films shares (watched.csv, ratings.csv, watchlist.csv…).
CREATE TABLE films (
    id             INTEGER PRIMARY KEY,
    letterboxd_uri TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL,
    year           INTEGER
) STRICT;

-- Films marked as watched. watched_on is the day the film was marked, as an
-- ISO 8601 date (YYYY-MM-DD).
CREATE TABLE watched (
    film_id    INTEGER PRIMARY KEY REFERENCES films (id) ON DELETE CASCADE,
    watched_on TEXT NOT NULL
) STRICT;

CREATE INDEX watched_by_date ON watched (watched_on DESC);
