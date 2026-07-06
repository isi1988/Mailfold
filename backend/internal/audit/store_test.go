package audit

import (
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open("sqlite", t.TempDir()+"/audit.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestRecordAndList(t *testing.T) {
	st := openTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	if err := st.Record(Entry{At: now, Actor: "admin", ActorType: "admin", Action: "login", Status: 200, IP: "1.2.3.4"}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := st.Record(Entry{At: now.Add(time.Second), Actor: "da1", ActorType: "domain_admin", Action: "login_failed", Status: 401, IP: "5.6.7.8"}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	entries, total, err := st.List(50, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 || len(entries) != 2 {
		t.Fatalf("total=%d len(entries)=%d, want 2, 2", total, len(entries))
	}
	// Newest first.
	if entries[0].Actor != "da1" || entries[0].ActorType != "domain_admin" || entries[0].Action != "login_failed" || entries[0].Status != 401 || entries[0].IP != "5.6.7.8" {
		t.Errorf("unexpected newest entry: %+v", entries[0])
	}
	if entries[1].Actor != "admin" || entries[1].Action != "login" {
		t.Errorf("unexpected oldest entry: %+v", entries[1])
	}
}

func TestListPagination(t *testing.T) {
	st := openTestStore(t)
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		if err := st.Record(Entry{At: now.Add(time.Duration(i) * time.Second), Actor: "admin", ActorType: "admin", Action: "POST /api/mailboxes", Status: 200, IP: "1.1.1.1"}); err != nil {
			t.Fatalf("Record %d: %v", i, err)
		}
	}
	entries, total, err := st.List(2, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 5 || len(entries) != 2 {
		t.Fatalf("total=%d len(entries)=%d, want 5, 2", total, len(entries))
	}
	entries2, total2, err := st.List(2, 2)
	if err != nil {
		t.Fatalf("List (offset 2): %v", err)
	}
	if total2 != 5 || len(entries2) != 2 {
		t.Fatalf("total2=%d len(entries2)=%d, want 5, 2", total2, len(entries2))
	}
	if entries[0].ID == entries2[0].ID {
		t.Error("paginated pages should not overlap")
	}
}

func TestListEmpty(t *testing.T) {
	st := openTestStore(t)
	entries, total, err := st.List(50, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 0 || len(entries) != 0 {
		t.Fatalf("total=%d len(entries)=%d, want 0, 0", total, len(entries))
	}
}
