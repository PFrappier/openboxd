-- The diary: one entry per viewing of a film, so a film watched again has
-- several of them.
CREATE TABLE diary_entries (
    id             INTEGER PRIMARY KEY,
    film_id        INTEGER NOT NULL REFERENCES films (id) ON DELETE CASCADE,
    -- URI of the entry on Letterboxd, not of the film: it tells the entries of
    -- an export apart, so that importing it again doesn't duplicate them.
    letterboxd_uri TEXT UNIQUE,
    -- The day the film was watched, as an ISO 8601 date (YYYY-MM-DD).
    watched_on     TEXT NOT NULL,
    -- The rating given along with this viewing, in half stars as in ratings.
    half_stars     INTEGER CHECK (half_stars BETWEEN 1 AND 10),
    -- Whether the film had been watched before.
    rewatch        INTEGER NOT NULL DEFAULT 0 CHECK (rewatch IN (0, 1))
) STRICT;

CREATE INDEX diary_entries_by_date ON diary_entries (watched_on DESC);
CREATE INDEX diary_entries_by_film ON diary_entries (film_id);
