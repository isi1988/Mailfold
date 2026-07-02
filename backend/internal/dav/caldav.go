package dav

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"
)

const defaultCalendarID = "default"

// NewCalDAVHandler builds a CalDAV HTTP handler backed by the store. prefix is
// the URL path the handler is mounted at (for example "/dav/caldav").
func NewCalDAVHandler(store *Store, prefix string) *caldav.Handler {
	return &caldav.Handler{Backend: &calBackend{store: store, pathHelper: pathHelper{prefix: prefix}}, Prefix: prefix}
}

// calBackend implements caldav.Backend on top of the SQLite store, scoping all
// data to the authenticated user taken from the request context.
type calBackend struct {
	store *Store
	pathHelper
}

// CurrentUserPrincipal returns the principal URL for the authenticated user.
func (b *calBackend) CurrentUserPrincipal(ctx context.Context) (string, error) {
	return b.principal(userFrom(ctx)), nil
}

// CalendarHomeSetPath returns the collection holding the user's calendars.
func (b *calBackend) CalendarHomeSetPath(ctx context.Context) (string, error) {
	return b.userHome(userFrom(ctx)), nil
}

func (b *calBackend) toCalendar(user string, c Book) caldav.Calendar {
	name := c.Name
	if name == "" {
		name = "Calendar"
	}
	return caldav.Calendar{
		Path:                  b.collPath(user, c.ID),
		Name:                  name,
		Description:           c.Description,
		MaxResourceSize:       1 << 20,
		SupportedComponentSet: []string{ical.CompEvent, ical.CompToDo},
	}
}

// ListCalendars returns the user's calendars, ensuring a default exists.
func (b *calBackend) ListCalendars(ctx context.Context) ([]caldav.Calendar, error) {
	user := userFrom(ctx)
	if err := b.store.EnsureCalendar(user, defaultCalendarID, "Calendar"); err != nil {
		return nil, err
	}
	cals, err := b.store.ListCalendars(user)
	if err != nil {
		return nil, err
	}
	out := make([]caldav.Calendar, 0, len(cals))
	for _, c := range cals {
		out = append(out, b.toCalendar(user, c))
	}
	return out, nil
}

// GetCalendar returns a single calendar.
func (b *calBackend) GetCalendar(ctx context.Context, path string) (*caldav.Calendar, error) {
	user := userFrom(ctx)
	id, err := b.collID(user, path)
	if err != nil {
		return nil, err
	}
	c, err := b.store.GetCalendar(user, id)
	if err != nil || c == nil {
		return nil, err
	}
	cal := b.toCalendar(user, *c)
	return &cal, nil
}

// CreateCalendar creates a new calendar from the request.
func (b *calBackend) CreateCalendar(ctx context.Context, cal *caldav.Calendar) error {
	user := userFrom(ctx)
	id, err := b.collID(user, cal.Path)
	if err != nil {
		return err
	}
	return b.store.CreateCalendar(user, Book{ID: id, Name: cal.Name, Description: cal.Description})
}

func (b *calBackend) toObject(user, calID string, o Object) (caldav.CalendarObject, error) {
	cal, err := ical.NewDecoder(strings.NewReader(o.Data)).Decode()
	if err != nil {
		return caldav.CalendarObject{}, err
	}
	return caldav.CalendarObject{
		Path:          b.objectPath(user, calID, o.UID, ".ics"),
		ModTime:       o.Modified,
		ContentLength: int64(len(o.Data)),
		ETag:          o.ETag,
		Data:          cal,
	}, nil
}

// GetCalendarObject returns a single iCalendar object.
func (b *calBackend) GetCalendarObject(ctx context.Context, path string, _ *caldav.CalendarCompRequest) (*caldav.CalendarObject, error) {
	user := userFrom(ctx)
	calID, uid, err := b.objectRef(user, path)
	if err != nil {
		return nil, err
	}
	o, err := b.store.GetCalObject(user, calID, uid)
	if err != nil || o == nil {
		return nil, err
	}
	co, err := b.toObject(user, calID, *o)
	if err != nil {
		return nil, err
	}
	return &co, nil
}

// ListCalendarObjects returns every object in a calendar.
func (b *calBackend) ListCalendarObjects(ctx context.Context, path string, _ *caldav.CalendarCompRequest) ([]caldav.CalendarObject, error) {
	user := userFrom(ctx)
	id, err := b.collID(user, path)
	if err != nil {
		return nil, err
	}
	objs, err := b.store.ListCalObjects(user, id)
	if err != nil {
		return nil, err
	}
	out := make([]caldav.CalendarObject, 0, len(objs))
	for _, o := range objs {
		co, err := b.toObject(user, id, o)
		if err != nil {
			return nil, err
		}
		out = append(out, co)
	}
	return out, nil
}

// QueryCalendarObjects lists objects and applies the client's filter.
func (b *calBackend) QueryCalendarObjects(ctx context.Context, path string, query *caldav.CalendarQuery) ([]caldav.CalendarObject, error) {
	all, err := b.ListCalendarObjects(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	return caldav.Filter(query, all)
}

// PutCalendarObject stores (creates or replaces) an iCalendar object (event or
// task).
func (b *calBackend) PutCalendarObject(ctx context.Context, path string, cal *ical.Calendar, _ *caldav.PutCalendarObjectOptions) (*caldav.CalendarObject, error) {
	user := userFrom(ctx)
	calID, uid, err := b.objectRef(user, path)
	if err != nil {
		return nil, err
	}
	if err := b.store.EnsureCalendar(user, calID, "Calendar"); err != nil {
		return nil, err
	}

	// Fill in mandatory properties the encoder requires so objects from lenient
	// clients still store cleanly.
	ensureICalRequired(cal)
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		return nil, err
	}
	stored, err := b.store.PutCalObject(user, calID, uid, buf.String())
	if err != nil {
		return nil, err
	}
	co := caldav.CalendarObject{
		Path:          b.objectPath(user, calID, uid, ".ics"),
		ModTime:       stored.Modified,
		ContentLength: int64(len(stored.Data)),
		ETag:          stored.ETag,
		Data:          cal,
	}
	return &co, nil
}

// ensureICalRequired fills in the mandatory iCalendar properties (VERSION,
// PRODID, and a DTSTAMP on each event/task) so objects submitted without them
// still satisfy the encoder instead of failing the request.
func ensureICalRequired(cal *ical.Calendar) {
	if cal.Props.Get(ical.PropVersion) == nil {
		cal.Props.SetText(ical.PropVersion, "2.0")
	}
	if cal.Props.Get(ical.PropProductID) == nil {
		cal.Props.SetText(ical.PropProductID, "-//Mailfold//CalDAV//EN")
	}
	now := time.Now().UTC()
	for _, child := range cal.Children {
		if child.Name != ical.CompEvent && child.Name != ical.CompToDo {
			continue
		}
		if child.Props.Get(ical.PropDateTimeStamp) == nil {
			child.Props.SetDateTime(ical.PropDateTimeStamp, now)
		}
	}
}

// DeleteCalendarObject removes an iCalendar object.
func (b *calBackend) DeleteCalendarObject(ctx context.Context, path string) error {
	user := userFrom(ctx)
	calID, uid, err := b.objectRef(user, path)
	if err != nil {
		return err
	}
	return b.store.DeleteCalObject(user, calID, uid)
}
