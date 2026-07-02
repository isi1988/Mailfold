// Package dav implements a self-hosted CardDAV/CalDAV groupware server backed by
// a local SQLite database. It lets Mailfold store and sync contacts (and, later,
// calendars) for mailbox users without depending on SOGo.
package dav

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no cgo)
)

// Book is a stored address book (or calendar collection).
type Book struct {
	ID          string
	Name        string
	Description string
}

// Object is a stored DAV object (a vCard or iCalendar resource).
type Object struct {
	UID      string
	ETag     string
	Data     string
	Modified time.Time
}

// Store is the SQLite-backed persistence layer for the DAV server.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path and applies the
// schema. It enables WAL mode and a busy timeout for safe concurrent access.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite writes are serialized; keep it simple
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS address_books (
    user TEXT NOT NULL, id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (user, id)
);
CREATE TABLE IF NOT EXISTS address_objects (
    user TEXT NOT NULL, book_id TEXT NOT NULL, uid TEXT NOT NULL,
    etag TEXT NOT NULL, data TEXT NOT NULL, modified INTEGER NOT NULL,
    PRIMARY KEY (user, book_id, uid)
);
CREATE TABLE IF NOT EXISTS calendars (
    user TEXT NOT NULL, id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (user, id)
);
CREATE TABLE IF NOT EXISTS calendar_objects (
    user TEXT NOT NULL, calendar_id TEXT NOT NULL, uid TEXT NOT NULL,
    etag TEXT NOT NULL, data TEXT NOT NULL, modified INTEGER NOT NULL,
    PRIMARY KEY (user, calendar_id, uid)
);`
	_, err := s.db.Exec(schema)
	return err
}

// etagOf derives a stable ETag from an object's content.
func etagOf(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:8])
}

// ---- address books ----

// EnsureBook creates the named address book for user if it does not exist.
func (s *Store) EnsureBook(user, id, name string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO address_books (user, id, name) VALUES (?, ?, ?)`,
		user, id, name)
	return err
}

// ListBooks returns the user's address books.
func (s *Store) ListBooks(user string) ([]Book, error) {
	rows, err := s.db.Query(`SELECT id, name, description FROM address_books WHERE user = ? ORDER BY id`, user)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Book
	for rows.Next() {
		var b Book
		if err := rows.Scan(&b.ID, &b.Name, &b.Description); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetBook returns one address book, or nil if absent.
func (s *Store) GetBook(user, id string) (*Book, error) {
	var b Book
	err := s.db.QueryRow(`SELECT id, name, description FROM address_books WHERE user = ? AND id = ?`, user, id).
		Scan(&b.ID, &b.Name, &b.Description)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// CreateBook creates an address book.
func (s *Store) CreateBook(user string, b Book) error {
	_, err := s.db.Exec(
		`INSERT INTO address_books (user, id, name, description) VALUES (?, ?, ?, ?)`,
		user, b.ID, b.Name, b.Description)
	return err
}

// DeleteBook removes an address book and all its objects.
func (s *Store) DeleteBook(user, id string) error {
	if _, err := s.db.Exec(`DELETE FROM address_objects WHERE user = ? AND book_id = ?`, user, id); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM address_books WHERE user = ? AND id = ?`, user, id)
	return err
}

// ---- address objects ----

// ListObjects returns every object in an address book.
func (s *Store) ListObjects(user, bookID string) ([]Object, error) {
	rows, err := s.db.Query(
		`SELECT uid, etag, data, modified FROM address_objects WHERE user = ? AND book_id = ? ORDER BY uid`,
		user, bookID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Object
	for rows.Next() {
		o, err := scanObject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// GetObject returns one object, or nil if absent.
func (s *Store) GetObject(user, bookID, uid string) (*Object, error) {
	row := s.db.QueryRow(
		`SELECT uid, etag, data, modified FROM address_objects WHERE user = ? AND book_id = ? AND uid = ?`,
		user, bookID, uid)
	o, err := scanObject(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// PutObject inserts or updates an object, returning the stored value with its
// freshly computed ETag and modification time.
func (s *Store) PutObject(user, bookID, uid, data string) (Object, error) {
	o := Object{UID: uid, ETag: etagOf(data), Data: data, Modified: time.Now().UTC()}
	_, err := s.db.Exec(
		`INSERT INTO address_objects (user, book_id, uid, etag, data, modified) VALUES (?, ?, ?, ?, ?, ?)
         ON CONFLICT(user, book_id, uid) DO UPDATE SET etag = excluded.etag, data = excluded.data, modified = excluded.modified`,
		user, bookID, uid, o.ETag, o.Data, o.Modified.Unix())
	if err != nil {
		return Object{}, err
	}
	return o, nil
}

// DeleteObject removes one object.
func (s *Store) DeleteObject(user, bookID, uid string) error {
	res, err := s.db.Exec(`DELETE FROM address_objects WHERE user = ? AND book_id = ? AND uid = ?`, user, bookID, uid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("object %q not found", uid)
	}
	return nil
}

// scanner abstracts *sql.Row and *sql.Rows for scanObject.
type scanner interface {
	Scan(dest ...any) error
}

func scanObject(sc scanner) (Object, error) {
	var o Object
	var mod int64
	if err := sc.Scan(&o.UID, &o.ETag, &o.Data, &mod); err != nil {
		return Object{}, err
	}
	o.Modified = time.Unix(mod, 0).UTC()
	return o, nil
}
