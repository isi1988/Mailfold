package dav

import (
	"context"
	"testing"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"
)

// sampleEvent builds a minimal VEVENT calendar with the given UID.
func sampleEvent(uid, summary string) *ical.Calendar {
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "-//Mailfold//Test//EN")

	ev := ical.NewComponent(ical.CompEvent)
	ev.Props.SetText(ical.PropUID, uid)
	ev.Props.SetText(ical.PropSummary, summary)
	ev.Props.SetDateTime(ical.PropDateTimeStart, time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC))
	ev.Props.SetDateTime(ical.PropDateTimeEnd, time.Date(2026, 7, 2, 11, 0, 0, 0, time.UTC))
	cal.Children = append(cal.Children, ev)
	return cal
}

func TestCalBackendRoundTrip(t *testing.T) {
	b := &calBackend{store: openTestStore(t), pathHelper: pathHelper{prefix: "/dav/caldav"}}
	ctx := WithUser(context.Background(), "user@example.com")

	if p, _ := b.CurrentUserPrincipal(ctx); p != "/dav/caldav/principals/user@example.com/" {
		t.Errorf("principal = %q", p)
	}
	home, _ := b.CalendarHomeSetPath(ctx)
	if home != "/dav/caldav/user@example.com/" {
		t.Errorf("home = %q", home)
	}

	// Listing creates the default calendar.
	cals, err := b.ListCalendars(ctx)
	if err != nil || len(cals) != 1 {
		t.Fatalf("ListCalendars: err=%v n=%d", err, len(cals))
	}
	if got := cals[0].SupportedComponentSet; len(got) != 2 {
		t.Errorf("SupportedComponentSet = %v", got)
	}
	calPath := cals[0].Path

	objPath := calPath + "meeting.ics"
	co, err := b.PutCalendarObject(ctx, objPath, sampleEvent("meeting", "Standup"), nil)
	if err != nil {
		t.Fatalf("PutCalendarObject: %v", err)
	}
	if co.ETag == "" {
		t.Fatalf("PutCalendarObject: empty etag")
	}

	got, err := b.GetCalendarObject(ctx, objPath, nil)
	if err != nil || got == nil {
		t.Fatalf("GetCalendarObject: err=%v", err)
	}
	if ev := got.Data.Events(); len(ev) != 1 || ev[0].Props.Get(ical.PropSummary).Value != "Standup" {
		t.Errorf("event summary mismatch: %+v", ev)
	}

	objs, err := b.ListCalendarObjects(ctx, calPath, nil)
	if err != nil || len(objs) != 1 {
		t.Fatalf("ListCalendarObjects: err=%v n=%d", err, len(objs))
	}
	if _, err := b.QueryCalendarObjects(ctx, calPath, &caldav.CalendarQuery{}); err != nil {
		t.Fatalf("QueryCalendarObjects: %v", err)
	}

	if err := b.DeleteCalendarObject(ctx, objPath); err != nil {
		t.Fatalf("DeleteCalendarObject: %v", err)
	}
	if got, _ := b.GetCalendarObject(ctx, objPath, nil); got != nil {
		t.Error("object should be gone after delete")
	}

	// Calendar create/get.
	if err := b.CreateCalendar(ctx, &caldav.Calendar{Path: home + "team/", Name: "Team"}); err != nil {
		t.Fatalf("CreateCalendar: %v", err)
	}
	if c, _ := b.GetCalendar(ctx, home+"team/"); c == nil {
		t.Error("GetCalendar returned nil for created calendar")
	}
}
