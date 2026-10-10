// Package database opens the SQLite database and keeps its schema up to date.
package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"
	"slices"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open opens the database stored at path, creating it if needed, and applies
// the migrations it is missing.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	pragmas := url.Values{"_pragma": {
		"foreign_keys(1)",
		// Readers don't block the writer, and the other way around.
		"journal_mode(WAL)",
		// Wait for a concurrent write to finish instead of failing right away.
		"busy_timeout(5000)",
	}}
	db, err := sql.Open("sqlite", "file:"+path+"?"+pragmas.Encode())
	if err != nil {
		return nil, err
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate %s: %w", path, err)
	}
	return db, nil
}

// migrate applies, in file name order, the migrations that come after the
// version recorded in the database (SQLite's user_version).
func migrate(ctx context.Context, db *sql.DB) error {
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	slices.Sort(names)

	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}

	for i, name := range names {
		if i < version {
			continue
		}
		if err := apply(ctx, db, name, i+1); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func apply(ctx context.Context, db *sql.DB, name string, version int) error {
	script, err := migrations.ReadFile(name)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, string(script)); err != nil {
		return err
	}
	// PRAGMA doesn't take parameters; version is ours, not user input.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		return err
	}
	return tx.Commit()
}
