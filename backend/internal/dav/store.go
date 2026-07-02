// Package dav implements a self-hosted CardDAV/CalDAV groupware server backed by
// a local database. It lets Mailfold store and sync contacts and calendars for
// mailbox users without depending on SOGo.
package dav

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/isi1988/Mailfold/backend/storage"
)

// The owner column is named "user" in SQL. Because user is a reserved word in
// PostgreSQL (the enterprise driver) it is always double-quoted; SQLite accepts
// the same quoting and resolves it to the identical column, so the schema is
// portable across both engines without a rename.

// Book is a stored collection (an address book or a calendar).
type Book struct {
	ID          string
	Name        string
	Description string
}

// Object is a stored DAV object (a vCard or an iCalendar resource).
type Object struct {
	UID      string
	ETag     string
	Data     string
	Modified time.Time
}

// Store is the persistence layer for the DAV server. It runs on any database the
// storage package has a driver for; all SQL is written once and adapted through
// the dialect.
type Store struct {
	db *sql.DB
	d  storage.Dialect
}

// Open opens the DAV database on the given driver and DSN and applies the schema.
func Open(driver, dsn string) (*Store, error) {
	db, err := storage.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db.DB, d: db.Dialect}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

// exec/query/queryRow centralise placeholder rebinding so every statement is
// dialect-correct without repeating the Rebind call at each site.
func (s *Store) exec(q string, args ...any) (sql.Result, error) {
	return s.db.Exec(s.d.Rebind(q), args...)
}

func (s *Store) query(q string, args ...any) (*sql.Rows, error) {
	return s.db.Query(s.d.Rebind(q), args...)
}

func (s *Store) queryRow(q string, args ...any) *sql.Row {
	return s.db.QueryRow(s.d.Rebind(q), args...)
}

func (s *Store) migrate() error {
	schema := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS address_books (
    "user" TEXT NOT NULL, id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '',
    PRIMARY KEY ("user", id)
);
CREATE TABLE IF NOT EXISTS address_objects (
    "user" TEXT NOT NULL, book_id TEXT NOT NULL, uid TEXT NOT NULL,
    etag TEXT NOT NULL, data TEXT NOT NULL, modified %[1]s NOT NULL,
    PRIMARY KEY ("user", book_id, uid)
);
CREATE TABLE IF NOT EXISTS calendars (
    "user" TEXT NOT NULL, id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '',
    PRIMARY KEY ("user", id)
);
CREATE TABLE IF NOT EXISTS calendar_objects (
    "user" TEXT NOT NULL, calendar_id TEXT NOT NULL, uid TEXT NOT NULL,
    etag TEXT NOT NULL, data TEXT NOT NULL, modified %[1]s NOT NULL,
    PRIMARY KEY ("user", calendar_id, uid)
);`, s.d.IntType())
	_, err := s.db.Exec(schema)
	return err
}

// etagOf derives a stable ETag from an object's content.
func etagOf(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:8])
}

// collection names the tables backing one DAV collection type. Address books and
// calendars share the same shape, so all CRUD is written once and parameterized
// by these (hardcoded, non-user) table/column names.
type collection struct {
	books   string // collection table
	objects string // object table
	fk      string // foreign-key column on the object table
}

var (
	addressColl  = collection{books: "address_books", objects: "address_objects", fk: "book_id"}
	calendarColl = collection{books: "calendars", objects: "calendar_objects", fk: "calendar_id"}
)

func (s *Store) ensure(c collection, user, id, name string) error {
	_, err := s.exec(fmt.Sprintf(`INSERT INTO %s ("user", id, name) VALUES (?, ?, ?) ON CONFLICT ("user", id) DO NOTHING`, c.books), user, id, name)
	return err
}

func (s *Store) listCollections(c collection, user string) ([]Book, error) {
	rows, err := s.query(fmt.Sprintf(`SELECT id, name, description FROM %s WHERE "user" = ? ORDER BY id`, c.books), user)
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

func (s *Store) getCollection(c collection, user, id string) (*Book, error) {
	var b Book
	err := s.queryRow(fmt.Sprintf(`SELECT id, name, description FROM %s WHERE "user" = ? AND id = ?`, c.books), user, id).
		Scan(&b.ID, &b.Name, &b.Description)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Store) createCollection(c collection, user string, b Book) error {
	_, err := s.exec(fmt.Sprintf(`INSERT INTO %s ("user", id, name, description) VALUES (?, ?, ?, ?)`, c.books),
		user, b.ID, b.Name, b.Description)
	return err
}

func (s *Store) deleteCollection(c collection, user, id string) error {
	if _, err := s.exec(fmt.Sprintf(`DELETE FROM %s WHERE "user" = ? AND %s = ?`, c.objects, c.fk), user, id); err != nil {
		return err
	}
	_, err := s.exec(fmt.Sprintf(`DELETE FROM %s WHERE "user" = ? AND id = ?`, c.books), user, id)
	return err
}

func (s *Store) listObjectsIn(c collection, user, collID string) ([]Object, error) {
	rows, err := s.query(
		fmt.Sprintf(`SELECT uid, etag, data, modified FROM %s WHERE "user" = ? AND %s = ? ORDER BY uid`, c.objects, c.fk),
		user, collID)
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

func (s *Store) getObjectIn(c collection, user, collID, uid string) (*Object, error) {
	row := s.queryRow(
		fmt.Sprintf(`SELECT uid, etag, data, modified FROM %s WHERE "user" = ? AND %s = ? AND uid = ?`, c.objects, c.fk),
		user, collID, uid)
	o, err := scanObject(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *Store) putObjectIn(c collection, user, collID, uid, data string) (Object, error) {
	o := Object{UID: uid, ETag: etagOf(data), Data: data, Modified: time.Now().UTC()}
	query := fmt.Sprintf(`INSERT INTO %s ("user", %s, uid, etag, data, modified) VALUES (?, ?, ?, ?, ?, ?)
        ON CONFLICT("user", %s, uid) DO UPDATE SET etag = excluded.etag, data = excluded.data, modified = excluded.modified`,
		c.objects, c.fk, c.fk)
	if _, err := s.exec(query, user, collID, uid, o.ETag, o.Data, storage.Unix(o.Modified)); err != nil {
		return Object{}, err
	}
	return o, nil
}

func (s *Store) deleteObjectIn(c collection, user, collID, uid string) error {
	res, err := s.exec(fmt.Sprintf(`DELETE FROM %s WHERE "user" = ? AND %s = ? AND uid = ?`, c.objects, c.fk), user, collID, uid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("object %q not found", uid)
	}
	return nil
}

// ---- address book API ----

// EnsureBook creates the named address book for user if it does not exist.
func (s *Store) EnsureBook(user, id, name string) error { return s.ensure(addressColl, user, id, name) }

// ListBooks returns the user's address books.
func (s *Store) ListBooks(user string) ([]Book, error) { return s.listCollections(addressColl, user) }

// GetBook returns one address book, or nil if absent.
func (s *Store) GetBook(user, id string) (*Book, error) {
	return s.getCollection(addressColl, user, id)
}

// CreateBook creates an address book.
func (s *Store) CreateBook(user string, b Book) error {
	return s.createCollection(addressColl, user, b)
}

// DeleteBook removes an address book and all its objects.
func (s *Store) DeleteBook(user, id string) error { return s.deleteCollection(addressColl, user, id) }

// ListObjects returns every object in an address book.
func (s *Store) ListObjects(user, bookID string) ([]Object, error) {
	return s.listObjectsIn(addressColl, user, bookID)
}

// GetObject returns one address object, or nil if absent.
func (s *Store) GetObject(user, bookID, uid string) (*Object, error) {
	return s.getObjectIn(addressColl, user, bookID, uid)
}

// PutObject inserts or updates an address object.
func (s *Store) PutObject(user, bookID, uid, data string) (Object, error) {
	return s.putObjectIn(addressColl, user, bookID, uid, data)
}

// DeleteObject removes an address object.
func (s *Store) DeleteObject(user, bookID, uid string) error {
	return s.deleteObjectIn(addressColl, user, bookID, uid)
}

// ---- calendar API ----

// EnsureCalendar creates the named calendar for user if it does not exist.
func (s *Store) EnsureCalendar(user, id, name string) error {
	return s.ensure(calendarColl, user, id, name)
}

// ListCalendars returns the user's calendars.
func (s *Store) ListCalendars(user string) ([]Book, error) {
	return s.listCollections(calendarColl, user)
}

// GetCalendar returns one calendar, or nil if absent.
func (s *Store) GetCalendar(user, id string) (*Book, error) {
	return s.getCollection(calendarColl, user, id)
}

// CreateCalendar creates a calendar.
func (s *Store) CreateCalendar(user string, b Book) error {
	return s.createCollection(calendarColl, user, b)
}

// ListCalObjects returns every object in a calendar.
func (s *Store) ListCalObjects(user, calID string) ([]Object, error) {
	return s.listObjectsIn(calendarColl, user, calID)
}

// GetCalObject returns one calendar object, or nil if absent.
func (s *Store) GetCalObject(user, calID, uid string) (*Object, error) {
	return s.getObjectIn(calendarColl, user, calID, uid)
}

// PutCalObject inserts or updates a calendar object.
func (s *Store) PutCalObject(user, calID, uid, data string) (Object, error) {
	return s.putObjectIn(calendarColl, user, calID, uid, data)
}

// DeleteCalObject removes a calendar object.
func (s *Store) DeleteCalObject(user, calID, uid string) error {
	return s.deleteObjectIn(calendarColl, user, calID, uid)
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
	o.Modified = storage.FromUnix(mod)
	return o, nil
}
