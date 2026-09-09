// Package store implements the SQLite persistence layer: connection setup,
// versioned migrations, and repositories for every persisted entity.
package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

// busyTimeoutMillis bounds how long a statement waits for the database
// lock before returning SQLITE_BUSY.
const busyTimeoutMillis = 5000

// DB wraps a *sql.DB opened against the application's SQLite database with
// WAL journaling, foreign keys, and a busy timeout. The pool is capped at
// one connection: with a single writer process this avoids SQLITE_BUSY
// entirely instead of tuning retries around it.
type DB struct {
	*sql.DB
}

// Open opens (creating if necessary) the SQLite database at path with the
// pragmas required by the persistence design: WAL journal mode, foreign
// keys enabled, and a busy timeout.
func Open(ctx context.Context, path string) (*DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(%d)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)",
		path, busyTimeoutMillis,
	)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database %q: %w", path, err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping database %q: %w", path, err)
	}
	return &DB{DB: sqlDB}, nil
}
