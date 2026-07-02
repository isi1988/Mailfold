package dav

import "testing"

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open("sqlite", t.TempDir()+"/dav.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestStoreBooks(t *testing.T) {
	st := openTestStore(t)
	const u = "u@example.com"

	if err := st.EnsureBook(u, "default", "Contacts"); err != nil {
		t.Fatalf("EnsureBook: %v", err)
	}
	if err := st.EnsureBook(u, "default", "Contacts"); err != nil {
		t.Fatalf("EnsureBook (idempotent): %v", err)
	}
	books, err := st.ListBooks(u)
	if err != nil || len(books) != 1 {
		t.Fatalf("ListBooks: err=%v n=%d", err, len(books))
	}
	if bk, _ := st.GetBook(u, "default"); bk == nil {
		t.Fatal("GetBook returned nil for existing book")
	}
	if bk, _ := st.GetBook(u, "missing"); bk != nil {
		t.Fatal("GetBook returned a book for a missing id")
	}
	if err := st.CreateBook(u, Book{ID: "work", Name: "Work"}); err != nil {
		t.Fatalf("CreateBook: %v", err)
	}
	if err := st.DeleteBook(u, "work"); err != nil {
		t.Fatalf("DeleteBook: %v", err)
	}
}

func TestStoreObjects(t *testing.T) {
	st := openTestStore(t)
	const u = "u@example.com"

	o, err := st.PutObject(u, "default", "c1", "BEGIN:VCARD\nVERSION:4.0\nUID:c1\nEND:VCARD")
	if err != nil || o.ETag == "" {
		t.Fatalf("PutObject: err=%v etag=%q", err, o.ETag)
	}
	got, err := st.GetObject(u, "default", "c1")
	if err != nil || got == nil || got.Data == "" {
		t.Fatalf("GetObject: err=%v obj=%+v", err, got)
	}
	// Updating the content must change the ETag.
	o2, _ := st.PutObject(u, "default", "c1", "BEGIN:VCARD\nVERSION:4.0\nUID:c1\nFN:x\nEND:VCARD")
	if o2.ETag == o.ETag {
		t.Error("ETag should change when content changes")
	}
	objs, _ := st.ListObjects(u, "default")
	if len(objs) != 1 {
		t.Fatalf("ListObjects n=%d", len(objs))
	}
	if got, _ := st.GetObject(u, "default", "missing"); got != nil {
		t.Error("GetObject returned an object for a missing uid")
	}
	if err := st.DeleteObject(u, "default", "c1"); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	if err := st.DeleteObject(u, "default", "c1"); err == nil {
		t.Error("second DeleteObject should report not found")
	}
}
