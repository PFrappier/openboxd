-- Lists of films the user made. A list is identified by its Letterboxd URI.
CREATE TABLE lists (
    id             INTEGER PRIMARY KEY,
    letterboxd_uri TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL,
    -- Empty for a list without one.
    description    TEXT NOT NULL DEFAULT '',
    -- The day the list was made, as an ISO 8601 date (YYYY-MM-DD).
    created_on     TEXT NOT NULL
) STRICT;

CREATE INDEX lists_by_date ON lists (created_on DESC);

-- position keeps the films of a list in the order the user put them, from 1.
CREATE TABLE list_films (
    list_id  INTEGER NOT NULL REFERENCES lists (id) ON DELETE CASCADE,
    film_id  INTEGER NOT NULL REFERENCES films (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    PRIMARY KEY (list_id, film_id)
) STRICT;

CREATE INDEX list_films_by_position ON list_films (list_id, position);
CREATE INDEX list_films_by_film ON list_films (film_id);
