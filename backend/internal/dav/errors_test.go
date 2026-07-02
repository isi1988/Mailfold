package dav

import (
	"context"
	"errors"
	"testing"

	"github.com/emersion/go-ical"
)

func TestHandlersConstructable(t *testing.T) {
	store := openTestStore(t)
	if NewCardDAVHandler(store, "/dav/carddav") == nil {
		t.Error("NewCardDAVHandler returned nil")
	}
	if NewCalDAVHandler(store, "/dav/caldav") == nil {
		t.Error("NewCalDAVHandler returned nil")
	}
}

func TestPathParsingErrors(t *testing.T) {
	p := pathHelper{prefix: "/dav/caldav"}
	if _, err := p.collID("user@x", "/dav/caldav/user@x/"); !errors.Is(err, errInvalidPath) {
		t.Errorf("collID on home path err = %v, want errInvalidPath", err)
	}
	if _, _, err := p.objectRef("user@x", "/dav/caldav/user@x/onlybook/"); !errors.Is(err, errInvalidPath) {
		t.Errorf("objectRef with no object err = %v, want errInvalidPath", err)
	}
}

func TestCalBackendErrorPaths(t *testing.T) {
	b := &calBackend{store: openTestStore(t), pathHelper: pathHelper{prefix: "/dav/caldav"}}
	ctx := WithUser(context.Background(), "user@example.com")

	// Invalid object path is rejected.
	if _, err := b.GetCalendarObject(ctx, "/dav/caldav/user@example.com/", nil); !errors.Is(err, errInvalidPath) {
		t.Errorf("GetCalendarObject bad path err = %v", err)
	}
	if err := b.DeleteCalendarObject(ctx, "/dav/caldav/user@example.com/"); !errors.Is(err, errInvalidPath) {
		t.Errorf("DeleteCalendarObject bad path err = %v", err)
	}

	// Absent calendar returns (nil, nil).
	if c, err := b.GetCalendar(ctx, "/dav/caldav/user@example.com/missing/"); c != nil || err != nil {
		t.Errorf("GetCalendar(missing) = %v, %v", c, err)
	}

	// Storing a bare event exercises every ensureICalRequired branch (no
	// VERSION/PRODID/DTSTAMP present).
	bare := ical.NewCalendar()
	ev := ical.NewComponent(ical.CompEvent)
	ev.Props.SetText(ical.PropUID, "bare")
	bare.Children = append(bare.Children, ev)
	if _, err := b.PutCalendarObject(ctx, "/dav/caldav/user@example.com/default/bare.ics", bare, nil); err != nil {
		t.Fatalf("PutCalendarObject(bare): %v", err)
	}

	// Corrupt stored data surfaces a decode error on read.
	if _, err := b.store.PutCalObject("user@example.com", "default", "broken", "not a calendar"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.GetCalendarObject(ctx, "/dav/caldav/user@example.com/default/broken.ics", nil); err == nil {
		t.Error("GetCalendarObject on corrupt data should error")
	}
	if _, err := b.ListCalendarObjects(ctx, "/dav/caldav/user@example.com/default/", nil); err == nil {
		t.Error("ListCalendarObjects with corrupt data should error")
	}
}

func TestCardBackendErrorPaths(t *testing.T) {
	b := &cardBackend{store: openTestStore(t), pathHelper: pathHelper{prefix: "/dav/carddav"}}
	ctx := WithUser(context.Background(), "user@example.com")

	if _, err := b.GetAddressObject(ctx, "/dav/carddav/user@example.com/", nil); !errors.Is(err, errInvalidPath) {
		t.Errorf("GetAddressObject bad path err = %v", err)
	}
	if err := b.DeleteAddressObject(ctx, "/dav/carddav/user@example.com/"); !errors.Is(err, errInvalidPath) {
		t.Errorf("DeleteAddressObject bad path err = %v", err)
	}
	if bk, err := b.GetAddressBook(ctx, "/dav/carddav/user@example.com/missing/"); bk != nil || err != nil {
		t.Errorf("GetAddressBook(missing) = %v, %v", bk, err)
	}

	// Corrupt stored data surfaces a decode error on read.
	if _, err := b.store.PutObject("user@example.com", "default", "broken", "not a vcard"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.GetAddressObject(ctx, "/dav/carddav/user@example.com/default/broken.vcf", nil); err == nil {
		t.Error("GetAddressObject on corrupt data should error")
	}
	if _, err := b.ListAddressObjects(ctx, "/dav/carddav/user@example.com/default/", nil); err == nil {
		t.Error("ListAddressObjects with corrupt data should error")
	}
}
