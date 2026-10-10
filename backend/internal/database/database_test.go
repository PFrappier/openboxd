package database

import (
	"path/filepath"
	"testing"
)

func TestOpenMigratesOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	// The second Open must find the schema in place and not apply it again.
	for range 2 {
		db, err := Open(t.Context(), path)
		if err != nil {
			t.Fatal(err)
		}
		var version int
		if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
			t.Fatal(err)
		}
		if version == 0 {
			t.Error("user_version = 0, want the number of migrations")
		}
		if _, err := db.Exec("SELECT id, letterboxd_uri, name, year FROM films"); err != nil {
			t.Error(err)
		}
		db.Close()
	}
}

func TestForeignKeysAreEnforced(t *testing.T) {
	db, err := Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec("INSERT INTO watched (film_id, watched_on) VALUES (42, '2024-01-01')"); err == nil {
		t.Error("inserted a watched row for a film that doesn't exist")
	}
}
