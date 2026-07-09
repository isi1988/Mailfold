package scheduledsend

import (
	"testing"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/webmail"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open("sqlite", t.TempDir()+"/scheduledsend.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func sampleMsg() webmail.OutgoingMessage {
	return webmail.OutgoingMessage{
		To:      []string{"a@example.com"},
		Cc:      []string{"cc@example.com"},
		Bcc:     nil,
		Subject: "Hello",
		Text:    "hi there",
		HTML:    "<p>hi there</p>",
	}
}

func TestCreateAndListPending(t *testing.T) {
	st := openTestStore(t)
	owner := "alice@example.com"
	scheduledAt := time.Now().Add(10 * time.Second).Truncate(time.Second)

	id, err := st.Create(owner, sampleMsg(), scheduledAt)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == 0 {
		t.Fatal("expected a non-zero id")
	}

	list, err := st.ListPending(owner)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListPending = %d rows, want 1", len(list))
	}
	got := list[0]
	if got.ID != id {
		t.Errorf("ID = %d, want %d", got.ID, id)
	}
	if got.OwnerEmail != owner {
		t.Errorf("OwnerEmail = %q, want %q", got.OwnerEmail, owner)
	}
	if len(got.To) != 1 || got.To[0] != "a@example.com" {
		t.Errorf("To = %v", got.To)
	}
	if len(got.Cc) != 1 || got.Cc[0] != "cc@example.com" {
		t.Errorf("Cc = %v", got.Cc)
	}
	if len(got.Bcc) != 0 {
		t.Errorf("Bcc = %v, want empty", got.Bcc)
	}
	if got.Subject != "Hello" || got.Text != "hi there" || got.HTML != "<p>hi there</p>" {
		t.Errorf("body fields mismatch: %+v", got)
	}
	if got.Status != StatusPending {
		t.Errorf("Status = %q, want pending", got.Status)
	}
	if !got.ScheduledAt.Equal(scheduledAt) {
		t.Errorf("ScheduledAt = %v, want %v", got.ScheduledAt, scheduledAt)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

// ListPending must only return the current user's own rows, and must
// exclude terminal (sent/canceled/failed) rows while including 'sending'
// ones (a row the dispatcher has claimed but not yet finished with is still
// "pending" from the owner's point of view).
func TestListPendingScopingAndStatusFilter(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()

	idAlicePending, err := st.Create("alice@example.com", sampleMsg(), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	idAliceSent, err := st.Create("alice@example.com", sampleMsg(), now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.MarkSent(idAliceSent); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Create("bob@example.com", sampleMsg(), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	list, err := st.ListPending("alice@example.com")
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(list) != 1 || list[0].ID != idAlicePending {
		t.Fatalf("ListPending(alice) = %+v, want only the pending row", list)
	}

	// Claim the pending row (moves it to 'sending') and confirm it still
	// shows up for the owner.
	claimed, err := st.ClaimDue(now.Add(2*time.Minute), 10)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	found := false
	for _, c := range claimed {
		if c.ID == idAlicePending {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected alice's pending row to be claimed, got %+v", claimed)
	}
	list, err = st.ListPending("alice@example.com")
	if err != nil {
		t.Fatalf("ListPending after claim: %v", err)
	}
	if len(list) != 1 || list[0].Status != StatusSending {
		t.Fatalf("ListPending(alice) after claim = %+v, want one 'sending' row", list)
	}
}

// ListPending must order soonest-first.
func TestListPendingOrder(t *testing.T) {
	st := openTestStore(t)
	owner := "alice@example.com"
	now := time.Now()
	idLater, err := st.Create(owner, sampleMsg(), now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	idSooner, err := st.Create(owner, sampleMsg(), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	list, err := st.ListPending(owner)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(list) != 2 || list[0].ID != idSooner || list[1].ID != idLater {
		t.Fatalf("ListPending order = %+v, want [sooner, later]", list)
	}
}

func TestCancelHappyPath(t *testing.T) {
	st := openTestStore(t)
	owner := "alice@example.com"
	id, err := st.Create(owner, sampleMsg(), time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	ok, err := st.Cancel(owner, id)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !ok {
		t.Fatal("Cancel should succeed on a pending row owned by the caller")
	}
	list, err := st.ListPending(owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("canceled row should no longer be pending, got %+v", list)
	}
}

func TestCancelWrongOwnerFails(t *testing.T) {
	st := openTestStore(t)
	id, err := st.Create("alice@example.com", sampleMsg(), time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	ok, err := st.Cancel("mallory@example.com", id)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if ok {
		t.Fatal("Cancel must not succeed for a non-owning caller")
	}
	// The row must be untouched.
	list, err := st.ListPending("alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Status != StatusPending {
		t.Fatalf("row should remain pending after a failed cross-owner cancel, got %+v", list)
	}
}

func TestCancelNonexistentFails(t *testing.T) {
	st := openTestStore(t)
	ok, err := st.Cancel("alice@example.com", 999999)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if ok {
		t.Fatal("Cancel of a nonexistent id must report ok=false, not error")
	}
}

// Cancel must refuse a row that is already 'sending' (claimed by the
// dispatcher) — it may only transition status='pending' -> 'canceled'.
func TestCancelRefusesAlreadySending(t *testing.T) {
	st := openTestStore(t)
	owner := "alice@example.com"
	id, err := st.Create(owner, sampleMsg(), time.Now().Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ClaimDue(time.Now(), 10)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != id {
		t.Fatalf("expected the row to be claimed, got %+v", claimed)
	}

	ok, err := st.Cancel(owner, id)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if ok {
		t.Fatal("Cancel must refuse a row that is already 'sending'")
	}
}

// Cancel must refuse rows in every terminal status: sent, canceled, failed.
func TestCancelRefusesTerminalStatuses(t *testing.T) {
	st := openTestStore(t)
	owner := "alice@example.com"

	idSent, err := st.Create(owner, sampleMsg(), time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.MarkSent(idSent); err != nil {
		t.Fatal(err)
	}
	if ok, err := st.Cancel(owner, idSent); err != nil || ok {
		t.Fatalf("Cancel on sent row: ok=%v err=%v, want ok=false", ok, err)
	}

	idCanceled, err := st.Create(owner, sampleMsg(), time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := st.Cancel(owner, idCanceled); err != nil || !ok {
		t.Fatalf("first cancel should succeed: ok=%v err=%v", ok, err)
	}
	if ok, err := st.Cancel(owner, idCanceled); err != nil || ok {
		t.Fatalf("second cancel of an already-canceled row: ok=%v err=%v, want ok=false", ok, err)
	}

	idFailed, err := st.Create(owner, sampleMsg(), time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.MarkFailed(idFailed, "boom"); err != nil {
		t.Fatal(err)
	}
	if ok, err := st.Cancel(owner, idFailed); err != nil || ok {
		t.Fatalf("Cancel on failed row: ok=%v err=%v, want ok=false", ok, err)
	}
}

func TestClaimDueOnlyClaimsDueRows(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()
	dueID, err := st.Create("alice@example.com", sampleMsg(), now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	futureID, err := st.Create("alice@example.com", sampleMsg(), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	claimed, err := st.ClaimDue(now, 10)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != dueID {
		t.Fatalf("ClaimDue = %+v, want only the due row (id=%d)", claimed, dueID)
	}
	if claimed[0].Status != StatusSending {
		t.Errorf("claimed row status = %q, want 'sending'", claimed[0].Status)
	}

	// The future row must remain untouched (still pending).
	list, err := st.ListPending("alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	var futureStillPending bool
	for _, r := range list {
		if r.ID == futureID && r.Status == StatusPending {
			futureStillPending = true
		}
	}
	if !futureStillPending {
		t.Fatalf("future row should remain pending, got %+v", list)
	}
}

func TestClaimDueRespectsLimit(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()
	for i := 0; i < 5; i++ {
		if _, err := st.Create("alice@example.com", sampleMsg(), now.Add(-time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	claimed, err := st.ClaimDue(now, 3)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if len(claimed) != 3 {
		t.Fatalf("ClaimDue with limit=3 returned %d rows", len(claimed))
	}
}

func TestClaimDueOrdersSoonestFirst(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()
	idLater, err := st.Create("alice@example.com", sampleMsg(), now.Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	idSooner, err := st.Create("alice@example.com", sampleMsg(), now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ClaimDue(now, 10)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if len(claimed) != 2 || claimed[0].ID != idSooner || claimed[1].ID != idLater {
		t.Fatalf("ClaimDue order = %+v, want [sooner, later]", claimed)
	}
}

// The critical safety property: a row claimed on one tick must never be
// claimable again on a later tick, even if the "send" for it is still in
// flight (simulated here by simply not marking it sent/failed between the
// two ClaimDue calls) — this is what guarantees at-most-once dispatch.
func TestClaimDueNeverDoubleClaims(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()
	id, err := st.Create("alice@example.com", sampleMsg(), now.Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}

	firstTick, err := st.ClaimDue(now, 10)
	if err != nil {
		t.Fatalf("first ClaimDue: %v", err)
	}
	if len(firstTick) != 1 || firstTick[0].ID != id {
		t.Fatalf("first tick should claim the row, got %+v", firstTick)
	}

	// Simulate a second, later tick before the first tick's dispatch call
	// has marked the row sent/failed (e.g. the process was slow).
	secondTick, err := st.ClaimDue(now.Add(time.Hour), 10)
	if err != nil {
		t.Fatalf("second ClaimDue: %v", err)
	}
	if len(secondTick) != 0 {
		t.Fatalf("second tick must not re-claim an already-claimed row, got %+v", secondTick)
	}

	// A third tick after the row is legitimately finished must also not
	// reclaim it (it's terminal now, not 'pending').
	if err := st.MarkSent(id); err != nil {
		t.Fatal(err)
	}
	thirdTick, err := st.ClaimDue(now.Add(2*time.Hour), 10)
	if err != nil {
		t.Fatalf("third ClaimDue: %v", err)
	}
	if len(thirdTick) != 0 {
		t.Fatalf("third tick must not claim a sent row, got %+v", thirdTick)
	}
}

func TestClaimDueNoRowsReturnsEmptyNotError(t *testing.T) {
	st := openTestStore(t)
	claimed, err := st.ClaimDue(time.Now(), 10)
	if err != nil {
		t.Fatalf("ClaimDue on empty table: %v", err)
	}
	if len(claimed) != 0 {
		t.Fatalf("expected no rows claimed, got %+v", claimed)
	}
}

func TestMarkSentAndMarkFailed(t *testing.T) {
	st := openTestStore(t)
	owner := "alice@example.com"
	idSent, err := st.Create(owner, sampleMsg(), time.Now().Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	idFailed, err := st.Create(owner, sampleMsg(), time.Now().Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimDue(time.Now(), 10); err != nil {
		t.Fatal(err)
	}

	if err := st.MarkSent(idSent); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}
	if err := st.MarkFailed(idFailed, "smtp said no"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	// Neither row should show up as pending/sending any longer.
	list, err := st.ListPending(owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("terminal rows should not be listed pending, got %+v", list)
	}
}

// ResetStale must only touch rows in 'sending' whose claimed_at predates the
// cutoff — a recently-claimed 'sending' row (still legitimately in flight)
// must be left alone, and a stale row in any other status must be left
// alone too.
func TestResetStaleOnlyTouchesSufficientlyStaleSendingRows(t *testing.T) {
	st := openTestStore(t)
	owner := "alice@example.com"
	now := time.Now()

	// A row claimed "just now" — still legitimately in flight.
	freshID, err := st.Create(owner, sampleMsg(), now.Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	// A row that will simulate having been claimed long ago and left
	// 'sending' (crash-orphaned).
	staleID, err := st.Create(owner, sampleMsg(), now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// A row that is stale in age but already terminal (sent) — must not be
	// touched by ResetStale (it only acts on 'sending' rows).
	staleSentID, err := st.Create(owner, sampleMsg(), now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := st.ClaimDue(now, 10); err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if err := st.MarkSent(staleSentID); err != nil {
		t.Fatal(err)
	}

	// Backdate staleID's claimed_at directly (simulating a row that was
	// claimed an hour ago and then the process crashed before MarkSent).
	if _, err := st.db.Exec(`UPDATE scheduled_send SET claimed_at = ? WHERE id = ?`, now.Add(-time.Hour).Unix(), staleID); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	n, err := st.ResetStale(now.Add(-5 * time.Minute))
	if err != nil {
		t.Fatalf("ResetStale: %v", err)
	}
	if n != 1 {
		t.Fatalf("ResetStale reset %d rows, want exactly 1 (the stale 'sending' row)", n)
	}

	// staleID must be back to pending.
	list, err := st.ListPending(owner)
	if err != nil {
		t.Fatal(err)
	}
	var staleBackToPending, freshUntouched bool
	for _, r := range list {
		if r.ID == staleID && r.Status == StatusPending {
			staleBackToPending = true
		}
		if r.ID == freshID && r.Status == StatusSending {
			freshUntouched = true
		}
	}
	if !staleBackToPending {
		t.Fatalf("stale row should be reset to pending, got %+v", list)
	}
	if !freshUntouched {
		t.Fatalf("fresh in-flight row should remain 'sending', got %+v", list)
	}
}

// Regression test for a real double-send bug found in review: ResetStale
// originally compared against created_at, which is unrelated to how long a
// row has actually been 'sending'. A "send later" message scheduled far in
// the future has an old created_at by the time it is legitimately claimed —
// so a startup ResetStale sweep with a several-minute cutoff would wrongly
// reset a row that had just been claimed (and possibly already sent)
// seconds earlier, causing the next dispatch tick to re-send it. This test
// creates exactly that scenario and asserts the row is left alone.
func TestResetStaleIgnoresOldCreatedAtOfARecentlyClaimedRow(t *testing.T) {
	st := openTestStore(t)
	owner := "bob@example.com"
	now := time.Now()

	// Scheduled 10 minutes out at creation time — created_at is "now",
	// scheduled_at is 10 minutes from now.
	id, err := st.Create(owner, sampleMsg(), now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	// The dispatcher claims it right on time, 10 minutes later — created_at
	// is now 10 minutes old, but claimed_at is fresh.
	claimTime := now.Add(10 * time.Minute)
	claimed, err := st.ClaimDue(claimTime, 10)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != id {
		t.Fatalf("ClaimDue = %+v, want exactly the scheduled row claimed", claimed)
	}

	// The process crashes before MarkSent runs. On restart, the startup
	// sweep calls ResetStale with a 5-minute cutoff relative to the crash
	// time (a few seconds after claimTime) — the old created_at-based logic
	// would treat this row (created 10+ minutes ago) as stale and reset it,
	// even though it was only just claimed.
	n, err := st.ResetStale(claimTime.Add(5 * time.Second).Add(-5 * time.Minute))
	if err != nil {
		t.Fatalf("ResetStale: %v", err)
	}
	if n != 0 {
		t.Fatalf("ResetStale reset %d rows, want 0 — a row claimed seconds ago must not be treated as stale just because it was created long before its scheduled time", n)
	}

	list, err := st.ListPending(owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Status != StatusSending {
		t.Fatalf("row should still be 'sending' after ResetStale, got %+v", list)
	}
}

func TestResetStaleNoStaleRowsReturnsZero(t *testing.T) {
	st := openTestStore(t)
	n, err := st.ResetStale(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("ResetStale: %v", err)
	}
	if n != 0 {
		t.Fatalf("ResetStale on empty table = %d, want 0", n)
	}
}

func TestOpenUnknownDriverFails(t *testing.T) {
	if _, err := Open("nope", "x"); err == nil {
		t.Error("unknown driver must error")
	}
}

func TestOpenBadDSNFails(t *testing.T) {
	// A DSN pointing at a path whose parent directory does not exist should
	// fail to open (SQLite cannot create the file).
	if _, err := Open("sqlite", "/nonexistent-dir-xyz/scheduledsend.db"); err == nil {
		t.Error("expected an error opening a database at a nonexistent directory")
	}
}

func TestCloseIsIdempotentSafe(t *testing.T) {
	st, err := Open("sqlite", t.TempDir()+"/close.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// Operations against a closed store must return an error rather than panic,
// exercising the error-return branches of Create/ListPending/Cancel/
// ClaimDue/ResetStale.
func TestOperationsOnClosedStoreError(t *testing.T) {
	st, err := Open("sqlite", t.TempDir()+"/closed.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := st.Create("alice@example.com", sampleMsg(), time.Now()); err == nil {
		t.Error("Create on a closed store should error")
	}
	if _, err := st.ListPending("alice@example.com"); err == nil {
		t.Error("ListPending on a closed store should error")
	}
	if _, err := st.Cancel("alice@example.com", 1); err == nil {
		t.Error("Cancel on a closed store should error")
	}
	if _, err := st.ClaimDue(time.Now(), 10); err == nil {
		t.Error("ClaimDue on a closed store should error")
	}
	if _, err := st.ResetStale(time.Now()); err == nil {
		t.Error("ResetStale on a closed store should error")
	}
}

func TestDecodeAddrListMalformedJSONReturnsNil(t *testing.T) {
	if got := decodeAddrList("not json"); got != nil {
		t.Errorf("decodeAddrList(malformed) = %v, want nil", got)
	}
	if got := decodeAddrList(""); got != nil {
		t.Errorf("decodeAddrList(empty) = %v, want nil", got)
	}
}

func TestEncodeDecodeAddrListRoundTrip(t *testing.T) {
	st := openTestStore(t)
	owner := "alice@example.com"
	msg := webmail.OutgoingMessage{To: []string{"a@example.com", "b@example.com"}}
	id, err := st.Create(owner, msg, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	list, err := st.ListPending(owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || len(list[0].To) != 2 {
		t.Fatalf("round trip mismatch: %+v", list)
	}
	_ = id
}
