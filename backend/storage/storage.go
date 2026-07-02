// Package storage is the database abstraction shared by Mailfold's SQLite-backed
// stores (DAV groupware and API keys). It is a thin driver registry plus a small
// SQL Dialect, so the same store code runs on more than one database without any
// per-database duplication: the open-source build registers SQLite here, and the
// enterprise build additionally registers PostgreSQL. The shared connection
// plumbing (WAL setup, single-connection policy) and the unix-time helpers also
// live here so each store is written once.
package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// Dialect abstracts the small SQL differences between databases so store code is
// written once. The open-source build ships only the SQLite dialect; the
// enterprise build adds a PostgreSQL dialect.
type Dialect interface {
	// Rebind converts '?' placeholders to the driver's parameter style.
	Rebind(query string) string
	// BlobType is the column type for binary data (BLOB vs BYTEA).
	BlobType() string
	// IntType is the column type for 64-bit integers such as unix timestamps
	// (INTEGER on SQLite, BIGINT where INTEGER would be 32-bit).
	IntType() string
}

// DB is an open database handle together with its dialect.
type DB struct {
	*sql.DB
	Dialect Dialect
}

// Opener opens a *sql.DB for a driver from its DSN, applying any driver-specific
// connection setup.
type Opener func(dsn string) (*sql.DB, error)

type entry struct {
	open    Opener
	dialect Dialect
}

var drivers = map[string]entry{}

// Register makes a database driver available under name. Drivers register
// themselves from an init function (SQLite in this package; PostgreSQL in the
// enterprise build), so which databases exist is decided at link time.
func Register(name string, open Opener, dialect Dialect) {
	drivers[name] = entry{open: open, dialect: dialect}
}

// Open opens the named driver. It returns a clear error when the driver is not
// compiled into this build — for example asking the open-source binary for
// "postgres", which is an enterprise-only driver.
func Open(driver, dsn string) (*DB, error) {
	e, ok := drivers[driver]
	if !ok {
		return nil, fmt.Errorf("storage: database driver %q is not available in this build", driver)
	}
	db, err := e.open(dsn)
	if err != nil {
		return nil, err
	}
	return &DB{DB: db, Dialect: e.dialect}, nil
}

// Unix encodes a time as a unix-second integer, mapping the zero time to 0.
func Unix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// FromUnix decodes a unix-second integer, mapping 0 back to the zero time.
func FromUnix(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(n, 0).UTC()
}
