package storage

import (
	"database/sql"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no cgo)
)

// DefaultDriver is the driver used when none is configured. SQLite ships with
// the open-source build; PostgreSQL is registered only by the enterprise build.
const DefaultDriver = "sqlite"

func init() {
	Register(DefaultDriver, openSQLite, sqliteDialect{})
}

// openSQLite opens the pure-Go SQLite database at the DSN (a file path), enabling
// WAL mode and a busy timeout, and serialises access to a single connection
// because SQLite is single-writer.
func openSQLite(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

type sqliteDialect struct{}

func (sqliteDialect) Rebind(q string) string { return q } // '?' is native to SQLite
func (sqliteDialect) BlobType() string       { return "BLOB" }
func (sqliteDialect) IntType() string        { return "INTEGER" }
