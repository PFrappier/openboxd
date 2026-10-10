-- Details that Letterboxd exports don't carry, fetched from TMDB after an
-- import. metadata_status is 'pending' until TMDB gave an answer about the
-- film, then 'matched' or 'not_found'.
ALTER TABLE films ADD COLUMN metadata_status TEXT NOT NULL DEFAULT 'pending'
    CHECK (metadata_status IN ('pending', 'matched', 'not_found'));
-- When TMDB was last asked about the film, as an RFC 3339 UTC timestamp.
ALTER TABLE films ADD COLUMN metadata_checked_at TEXT;
ALTER TABLE films ADD COLUMN tmdb_id INTEGER;
ALTER TABLE films ADD COLUMN overview TEXT;
-- Path of the poster on TMDB's image CDN, e.g. "/8Gxv8gSFCU0XGDykEGv7zR1n2ua.jpg".
ALTER TABLE films ADD COLUMN poster_path TEXT;
-- In minutes.
ALTER TABLE films ADD COLUMN runtime INTEGER;

-- The queue of films to look up: never asked first, then least recently asked.
CREATE INDEX films_pending_metadata ON films (metadata_checked_at, id)
    WHERE metadata_status = 'pending';

-- A person is identified by their TMDB ID.
CREATE TABLE people (
    id      INTEGER PRIMARY KEY,
    tmdb_id INTEGER NOT NULL UNIQUE,
    name    TEXT NOT NULL
) STRICT;

-- position keeps the directors of a film in the order TMDB credits them.
CREATE TABLE film_directors (
    film_id   INTEGER NOT NULL REFERENCES films (id) ON DELETE CASCADE,
    person_id INTEGER NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    position  INTEGER NOT NULL,
    PRIMARY KEY (film_id, person_id)
) STRICT;

CREATE INDEX film_directors_by_person ON film_directors (person_id);
