-- Films the user wants to watch. added_on is the day the film was added to
-- the watchlist, as an ISO 8601 date (YYYY-MM-DD).
CREATE TABLE watchlist (
    film_id  INTEGER PRIMARY KEY REFERENCES films (id) ON DELETE CASCADE,
    added_on TEXT NOT NULL
) STRICT;

CREATE INDEX watchlist_by_date ON watchlist (added_on DESC);
