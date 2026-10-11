-- Reviews the user wrote: one per review, so a film reviewed again has
-- several of them.
CREATE TABLE reviews (
    id             INTEGER PRIMARY KEY,
    film_id        INTEGER NOT NULL REFERENCES films (id) ON DELETE CASCADE,
    -- URI of the review on Letterboxd, not of the film: it tells the reviews of
    -- an export apart, so that importing it again doesn't duplicate them. The
    -- review of a logged viewing shares it with its diary entry.
    letterboxd_uri TEXT NOT NULL UNIQUE,
    -- The day the review was written, as an ISO 8601 date (YYYY-MM-DD).
    reviewed_on    TEXT NOT NULL,
    -- The day of the viewing the review is about; null for a review written
    -- without logging one.
    watched_on     TEXT,
    -- The rating given along with the review, in half stars as in ratings.
    half_stars     INTEGER CHECK (half_stars BETWEEN 1 AND 10),
    -- Whether the film had been watched before.
    rewatch        INTEGER NOT NULL DEFAULT 0 CHECK (rewatch IN (0, 1)),
    -- The text as written on Letterboxd, which allows a few HTML tags in it.
    body           TEXT NOT NULL
) STRICT;

CREATE INDEX reviews_by_date ON reviews (reviewed_on DESC);
CREATE INDEX reviews_by_film ON reviews (film_id);
