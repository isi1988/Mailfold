package dav

import (
	"context"
	"testing"

	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav/carddav"
)

func TestCardBackendRoundTrip(t *testing.T) {
	b := &cardBackend{store: openTestStore(t), pathHelper: pathHelper{prefix: "/dav/carddav"}}
	ctx := WithUser(context.Background(), "user@example.com")

	if p, _ := b.CurrentUserPrincipal(ctx); p != "/dav/carddav/principals/user@example.com/" {
		t.Errorf("principal = %q", p)
	}
	home, _ := b.AddressBookHomeSetPath(ctx)
	if home != "/dav/carddav/user@example.com/" {
		t.Errorf("home = %q", home)
	}

	// Listing creates the default book.
	books, err := b.ListAddressBooks(ctx)
	if err != nil || len(books) != 1 {
		t.Fatalf("ListAddressBooks: err=%v n=%d", err, len(books))
	}
	bookPath := books[0].Path

	card := vcard.Card{}
	card.SetValue(vcard.FieldFormattedName, "John Doe")
	card.SetValue(vcard.FieldUID, "john")
	objPath := bookPath + "john.vcf"

	ao, err := b.PutAddressObject(ctx, objPath, card, nil)
	if err != nil || ao.ETag == "" {
		t.Fatalf("PutAddressObject: err=%v etag=%q", err, ao.ETag)
	}

	got, err := b.GetAddressObject(ctx, objPath, nil)
	if err != nil || got == nil {
		t.Fatalf("GetAddressObject: err=%v", err)
	}
	if got.Card.Value(vcard.FieldFormattedName) != "John Doe" {
		t.Errorf("FN = %q", got.Card.Value(vcard.FieldFormattedName))
	}

	objs, err := b.ListAddressObjects(ctx, bookPath, nil)
	if err != nil || len(objs) != 1 {
		t.Fatalf("ListAddressObjects: err=%v n=%d", err, len(objs))
	}
	if _, err := b.QueryAddressObjects(ctx, bookPath, &carddav.AddressBookQuery{}); err != nil {
		t.Fatalf("QueryAddressObjects: %v", err)
	}

	if err := b.DeleteAddressObject(ctx, objPath); err != nil {
		t.Fatalf("DeleteAddressObject: %v", err)
	}
	if got, _ := b.GetAddressObject(ctx, objPath, nil); got != nil {
		t.Error("object should be gone after delete")
	}

	// Address-book create/get/delete.
	if err := b.CreateAddressBook(ctx, &carddav.AddressBook{Path: home + "work/", Name: "Work"}); err != nil {
		t.Fatalf("CreateAddressBook: %v", err)
	}
	if bk, _ := b.GetAddressBook(ctx, home+"work/"); bk == nil {
		t.Error("GetAddressBook returned nil for created book")
	}
	if err := b.DeleteAddressBook(ctx, home+"work/"); err != nil {
		t.Fatalf("DeleteAddressBook: %v", err)
	}
}
