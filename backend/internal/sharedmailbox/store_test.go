package sharedmailbox

import (
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open("sqlite", t.TempDir()+"/sharedmailbox.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestCreateGetListDeleteMailbox(t *testing.T) {
	st := openTestStore(t)
	if _, ok, err := st.GetMailboxByEmail("support@example.com"); err != nil || ok {
		t.Fatalf("unknown mailbox should not be found: ok=%v err=%v", ok, err)
	}

	id, err := st.CreateMailbox(Mailbox{
		Email: "support@example.com", DisplayName: "Support",
		AppPasswdID: "42", AppPasswdEnc: []byte("enc"), AppPasswdNonce: []byte("nonce"),
		CreatedBy: "admin",
	}, time.Now())
	if err != nil {
		t.Fatalf("CreateMailbox: %v", err)
	}

	m, ok, err := st.GetMailbox(id)
	if err != nil || !ok {
		t.Fatalf("GetMailbox: ok=%v err=%v", ok, err)
	}
	if m.Email != "support@example.com" || m.DisplayName != "Support" || m.AppPasswdID != "42" || m.CreatedBy != "admin" {
		t.Errorf("GetMailbox = %+v, unexpected fields", m)
	}

	byEmail, ok, err := st.GetMailboxByEmail("support@example.com")
	if err != nil || !ok || byEmail.ID != id {
		t.Fatalf("GetMailboxByEmail = %+v, ok=%v err=%v, want id=%d", byEmail, ok, err, id)
	}

	list, err := st.ListMailboxes()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListMailboxes = %+v, err=%v, want 1 entry", list, err)
	}

	if err := st.DeleteMailbox(id); err != nil {
		t.Fatalf("DeleteMailbox: %v", err)
	}
	if _, ok, _ := st.GetMailbox(id); ok {
		t.Error("mailbox should be gone after DeleteMailbox")
	}
}

func TestCreateMailboxDuplicateEmailFails(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.CreateMailbox(Mailbox{Email: "support@example.com"}, time.Now()); err != nil {
		t.Fatalf("first CreateMailbox: %v", err)
	}
	if _, err := st.CreateMailbox(Mailbox{Email: "support@example.com"}, time.Now()); err == nil {
		t.Error("expected an error creating a second shared mailbox for the same email")
	}
}

func TestMembers(t *testing.T) {
	st := openTestStore(t)
	id, err := st.CreateMailbox(Mailbox{Email: "support@example.com"}, time.Now())
	if err != nil {
		t.Fatalf("CreateMailbox: %v", err)
	}

	if ok, err := st.IsMember(id, "alice@example.com"); err != nil || ok {
		t.Fatalf("IsMember before add: ok=%v err=%v", ok, err)
	}
	if err := st.AddMember(id, "alice@example.com", time.Now()); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if err := st.AddMember(id, "bob@example.com", time.Now()); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	// Adding the same member again must not error (idempotent).
	if err := st.AddMember(id, "alice@example.com", time.Now()); err != nil {
		t.Fatalf("AddMember (repeat): %v", err)
	}

	if ok, err := st.IsMember(id, "alice@example.com"); err != nil || !ok {
		t.Fatalf("IsMember after add: ok=%v err=%v", ok, err)
	}
	members, err := st.ListMembers(id)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 2 || members[0] != "alice@example.com" || members[1] != "bob@example.com" {
		t.Fatalf("ListMembers = %v, want [alice@example.com bob@example.com]", members)
	}

	mine, err := st.MailboxesForMember("alice@example.com")
	if err != nil || len(mine) != 1 || mine[0].ID != id {
		t.Fatalf("MailboxesForMember = %+v, err=%v, want one mailbox with id=%d", mine, err, id)
	}
	if _, err := st.MailboxesForMember("nobody@example.com"); err != nil {
		t.Fatalf("MailboxesForMember for a stranger should not error: %v", err)
	}

	if err := st.RemoveMember(id, "bob@example.com"); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	members, _ = st.ListMembers(id)
	if len(members) != 1 || members[0] != "alice@example.com" {
		t.Fatalf("ListMembers after remove = %v, want [alice@example.com]", members)
	}
}

func TestAssignment(t *testing.T) {
	st := openTestStore(t)
	id, _ := st.CreateMailbox(Mailbox{Email: "support@example.com"}, time.Now())

	if _, ok, err := st.GetAssignment(id, "INBOX", 1); err != nil || ok {
		t.Fatalf("unassigned message should not be found: ok=%v err=%v", ok, err)
	}

	if err := st.SetAssignment(id, "INBOX", 1, "alice@example.com", "admin", time.Now()); err != nil {
		t.Fatalf("SetAssignment: %v", err)
	}
	assignedTo, ok, err := st.GetAssignment(id, "INBOX", 1)
	if err != nil || !ok || assignedTo != "alice@example.com" {
		t.Fatalf("GetAssignment = %q, ok=%v err=%v, want alice@example.com, true, nil", assignedTo, ok, err)
	}

	// Re-assigning overwrites rather than erroring on the existing row.
	if err := st.SetAssignment(id, "INBOX", 1, "bob@example.com", "admin", time.Now()); err != nil {
		t.Fatalf("SetAssignment (reassign): %v", err)
	}
	assignedTo, _, _ = st.GetAssignment(id, "INBOX", 1)
	if assignedTo != "bob@example.com" {
		t.Errorf("assignedTo after reassign = %q, want bob@example.com", assignedTo)
	}

	if err := st.SetAssignment(id, "INBOX", 2, "alice@example.com", "admin", time.Now()); err != nil {
		t.Fatalf("SetAssignment (second message): %v", err)
	}
	byFolder, err := st.AssignmentsForFolder(id, "INBOX")
	if err != nil {
		t.Fatalf("AssignmentsForFolder: %v", err)
	}
	if byFolder[1] != "bob@example.com" || byFolder[2] != "alice@example.com" {
		t.Fatalf("AssignmentsForFolder = %v, want {1:bob@example.com 2:alice@example.com}", byFolder)
	}

	if err := st.ClearAssignment(id, "INBOX", 1); err != nil {
		t.Fatalf("ClearAssignment: %v", err)
	}
	if _, ok, _ := st.GetAssignment(id, "INBOX", 1); ok {
		t.Error("assignment should be gone after ClearAssignment")
	}
}

func TestNotes(t *testing.T) {
	st := openTestStore(t)
	id, _ := st.CreateMailbox(Mailbox{Email: "support@example.com"}, time.Now())

	notes, err := st.ListNotes(id, "INBOX", 1)
	if err != nil || len(notes) != 0 {
		t.Fatalf("ListNotes on a message with no notes = %+v, err=%v, want empty", notes, err)
	}

	n1, err := st.AddNote(id, "INBOX", 1, "alice@example.com", "customer called back", time.Now())
	if err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	if n1.ID == 0 {
		t.Error("AddNote should return a non-zero id")
	}
	if _, err := st.AddNote(id, "INBOX", 1, "bob@example.com", "escalating to L2", time.Now()); err != nil {
		t.Fatalf("AddNote (second): %v", err)
	}

	notes, err = st.ListNotes(id, "INBOX", 1)
	if err != nil || len(notes) != 2 {
		t.Fatalf("ListNotes = %+v, err=%v, want 2 notes", notes, err)
	}
	if notes[0].AuthorEmail != "alice@example.com" || notes[1].AuthorEmail != "bob@example.com" {
		t.Fatalf("ListNotes order/authors = %+v, want alice then bob", notes)
	}

	got, ok, err := st.GetNote(n1.ID)
	if err != nil || !ok || got.Body != "customer called back" {
		t.Fatalf("GetNote = %+v, ok=%v err=%v", got, ok, err)
	}

	counts, err := st.NoteCountsForFolder(id, "INBOX")
	if err != nil || counts[1] != 2 {
		t.Fatalf("NoteCountsForFolder = %v, err=%v, want {1:2}", counts, err)
	}

	if err := st.DeleteNote(n1.ID); err != nil {
		t.Fatalf("DeleteNote: %v", err)
	}
	if _, ok, _ := st.GetNote(n1.ID); ok {
		t.Error("note should be gone after DeleteNote")
	}
	notes, _ = st.ListNotes(id, "INBOX", 1)
	if len(notes) != 1 {
		t.Fatalf("ListNotes after delete = %+v, want 1 remaining", notes)
	}
}

func TestDeleteMailboxCascades(t *testing.T) {
	st := openTestStore(t)
	id, _ := st.CreateMailbox(Mailbox{Email: "support@example.com"}, time.Now())
	_ = st.AddMember(id, "alice@example.com", time.Now())
	_ = st.SetAssignment(id, "INBOX", 1, "alice@example.com", "admin", time.Now())
	if _, err := st.AddNote(id, "INBOX", 1, "alice@example.com", "hi", time.Now()); err != nil {
		t.Fatalf("AddNote: %v", err)
	}

	if err := st.DeleteMailbox(id); err != nil {
		t.Fatalf("DeleteMailbox: %v", err)
	}

	if members, err := st.ListMembers(id); err != nil || len(members) != 0 {
		t.Errorf("ListMembers after DeleteMailbox = %v, err=%v, want empty", members, err)
	}
	if _, ok, err := st.GetAssignment(id, "INBOX", 1); err != nil || ok {
		t.Errorf("GetAssignment after DeleteMailbox: ok=%v err=%v, want false", ok, err)
	}
	if notes, err := st.ListNotes(id, "INBOX", 1); err != nil || len(notes) != 0 {
		t.Errorf("ListNotes after DeleteMailbox = %v, err=%v, want empty", notes, err)
	}
}
