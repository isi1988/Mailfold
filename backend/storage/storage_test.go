package storage

import (
	"testing"
	"time"
)

func TestOpenSQLite(t *testing.T) {
	db, err := Open("sqlite", t.TempDir()+"/t.db")
	if err != nil {
		t.Fatalf("Open sqlite: %v", err)
	}
	defer func() { _ = db.Close() }()
	if db.Dialect == nil {
		t.Fatal("dialect should be set")
	}
	if err := db.Ping(); err != nil {
		t.Errorf("ping: %v", err)
	}
}

func TestOpenUnknownDriver(t *testing.T) {
	if _, err := Open("nope", "x"); err == nil {
		t.Error("unknown driver must error (postgres is enterprise-only)")
	}
}

func TestSQLiteDialect(t *testing.T) {
	var d Dialect = sqliteDialect{}
	if d.Rebind("SELECT ?") != "SELECT ?" {
		t.Error("sqlite Rebind should be identity")
	}
	if d.BlobType() != "BLOB" || d.IntType() != "INTEGER" {
		t.Errorf("unexpected types: %s / %s", d.BlobType(), d.IntType())
	}
}

func TestUnixHelpers(t *testing.T) {
	if Unix(time.Time{}) != 0 {
		t.Error("zero time should encode to 0")
	}
	if !FromUnix(0).IsZero() {
		t.Error("0 should decode to the zero time")
	}
	now := time.Now().Truncate(time.Second)
	if got := FromUnix(Unix(now)); !got.Equal(now) {
		t.Errorf("round-trip mismatch: %v vs %v", got, now)
	}
}
