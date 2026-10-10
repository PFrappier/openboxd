-- The user's rating of a film, in half stars: from 1 (½) to 10 (★★★★★).
-- rated_on is an ISO 8601 date (YYYY-MM-DD).
CREATE TABLE ratings (
    film_id    INTEGER PRIMARY KEY REFERENCES films (id) ON DELETE CASCADE,
    half_stars INTEGER NOT NULL CHECK (half_stars BETWEEN 1 AND 10),
    rated_on   TEXT NOT NULL
) STRICT;

-- Films the user liked, which doesn't require having watched them.
CREATE TABLE likes (
    film_id  INTEGER PRIMARY KEY REFERENCES films (id) ON DELETE CASCADE,
    liked_on TEXT NOT NULL
) STRICT;
