package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// errEventNotFound is returned when an event UID does not resolve.
var errEventNotFound = errors.New("event not found")

// attachNameFmt names an attachment that carries no filename.
const attachNameFmt = "attachment-%d"

// webmailCalendarID is the single calendar collection each mailbox's webmail
// calendar uses. It is created on demand.
const webmailCalendarID = "default"

// attachFilenameParam carries an attachment's original filename alongside the
// inline binary ATTACH property.
const attachFilenameParam = "X-FILENAME"

// maxCalAttachTotal caps the combined decoded size of an event's attachments so
// a single VEVENT cannot bloat the SQLite store.
const maxCalAttachTotal = 10 << 20 // 10 MiB

// partstatParam is the iCalendar ATTENDEE participation-status parameter that
// carries the mailbox owner's RSVP.
const partstatParam = "PARTSTAT"

// mailtoPrefix is the URI scheme prefixing an ATTENDEE's email address.
const mailtoPrefix = "mailto:"

// eventAttachment is a file attached to a calendar event. Data (base64) is only
// carried inbound on create and outbound on a single-attachment fetch; the list
// endpoint returns metadata only.
type eventAttachment struct {
	Filename string `json:"filename"`
	Mime     string `json:"mime,omitempty"`
	Size     int    `json:"size,omitempty"`
	Data     string `json:"data,omitempty"`
}

// calendarEvent is the JSON shape the webmail calendar UI works with.
type calendarEvent struct {
	UID         string            `json:"uid"`
	Summary     string            `json:"summary"`
	Start       time.Time         `json:"start"`
	End         time.Time         `json:"end"`
	AllDay      bool              `json:"all_day"`
	Location    string            `json:"location,omitempty"`
	Description string            `json:"description,omitempty"`
	Calendar    string            `json:"calendar,omitempty"`
	Guests      []string          `json:"guests,omitempty"`
	Repeat      string            `json:"repeat,omitempty"`   // DAILY|WEEKLY|MONTHLY|YEARLY
	Reminder    int               `json:"reminder,omitempty"` // minutes before start; 0 = none
	Rsvp        string            `json:"rsvp,omitempty"`     // yes|maybe|no|none — the owner's response
	Attachments []eventAttachment `json:"attachments,omitempty"`
}

// registerWebmailCalendar wires the JSON calendar endpoints, which read and
// write the logged-in mailbox's events in the self-hosted CalDAV store. They are
// only mounted when the store is configured.
func (s *Server) registerWebmailCalendar(mux *http.ServeMux) {
	if s.davStore == nil {
		return
	}
	mux.HandleFunc("GET /api/webmail/calendar/events", s.requireWebmail(s.handleCalendarList))
	mux.HandleFunc("POST /api/webmail/calendar/events", s.requireWebmail(s.handleCalendarCreate))
	mux.HandleFunc("PUT /api/webmail/calendar/events/{uid}", s.requireWebmail(s.handleCalendarUpdate))
	mux.HandleFunc("DELETE /api/webmail/calendar/events/{uid}", s.requireWebmail(s.handleCalendarDelete))
	mux.HandleFunc("PATCH /api/webmail/calendar/events/{uid}/rsvp", s.requireWebmail(s.handleCalendarRsvp))
	mux.HandleFunc("GET /api/webmail/calendar/events/{uid}/attachments/{index}", s.requireWebmail(s.handleCalendarAttachment))
}

func (s *Server) handleCalendarList(w http.ResponseWriter, r *http.Request) {
	user := webmailCreds(r).Email
	if err := s.davStore.EnsureCalendar(user, webmailCalendarID, "Calendar"); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	objs, err := s.davStore.ListCalObjects(user, webmailCalendarID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	events := make([]calendarEvent, 0, len(objs))
	for _, o := range objs {
		if ev, ok := parseEvent(o.Data, user); ok {
			events = append(events, ev)
		}
	}
	writeJSON(w, http.StatusOK, events)
}

type createEventRequest struct {
	Summary     string            `json:"summary"`
	Start       time.Time         `json:"start"`
	End         time.Time         `json:"end"`
	AllDay      bool              `json:"all_day"`
	Location    string            `json:"location"`
	Description string            `json:"description"`
	Calendar    string            `json:"calendar"`
	Guests      []string          `json:"guests"`
	Repeat      string            `json:"repeat"`
	Reminder    int               `json:"reminder"`
	Rsvp        string            `json:"rsvp"`
	Attachments []eventAttachment `json:"attachments"`
}

func (s *Server) handleCalendarCreate(w http.ResponseWriter, r *http.Request) {
	user := webmailCreds(r).Email
	var req createEventRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Summary) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "summary is required"})
		return
	}
	if req.Start.IsZero() {
		req.Start = time.Now().UTC()
	}
	if !req.End.After(req.Start) {
		req.End = req.Start.Add(time.Hour)
	}
	total := 0
	for _, a := range req.Attachments {
		total += base64.StdEncoding.DecodedLen(len(a.Data))
	}
	if total > maxCalAttachTotal {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "attachments are too large"})
		return
	}
	uid := newEventUID()
	data := buildEvent(uid, req, user)
	if err := s.davStore.EnsureCalendar(user, webmailCalendarID, "Calendar"); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if _, err := s.davStore.PutCalObject(user, webmailCalendarID, uid, data); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"uid": uid})
}

// handleCalendarUpdate rewrites an event in place from the request while
// preserving its attachments and the owner's RSVP (which are not edited here).
// It serves both the edit form and drag-to-reschedule.
func (s *Server) handleCalendarUpdate(w http.ResponseWriter, r *http.Request) {
	user := webmailCreds(r).Email
	uid := r.PathValue("uid")
	var req createEventRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Summary) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "summary is required"})
		return
	}
	if req.Start.IsZero() {
		req.Start = time.Now().UTC()
	}
	if !req.End.After(req.Start) {
		req.End = req.Start.Add(time.Hour)
	}
	existing, err := s.davStore.GetCalObject(user, webmailCalendarID, uid)
	if err != nil || existing == nil {
		s.writeError(w, http.StatusNotFound, errEventNotFound)
		return
	}
	data, ok := editEvent(existing.Data, req, user)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, fmt.Errorf("could not update the event"))
		return
	}
	if _, err := s.davStore.PutCalObject(user, webmailCalendarID, uid, data); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"uid": uid})
}

// editEvent rewrites the user-editable fields of an existing VEVENT in place,
// preserving everything the webmail form does not model: attachments, the
// owner's RSVP, the recurrence detail (when the frequency is unchanged), other
// alarms, guest parameters, and any external properties (ORGANIZER, STATUS, …).
func editEvent(data string, req createEventRequest, owner string) (string, bool) {
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return "", false
	}
	var ev *ical.Component
	for _, ch := range cal.Children {
		if ch.Name == ical.CompEvent {
			ev = ch
			break
		}
	}
	if ev == nil {
		return "", false
	}
	ev.Props.SetText(ical.PropSummary, icalText(req.Summary))
	setOrDelete(ev, ical.PropLocation, req.Location)
	setOrDelete(ev, ical.PropDescription, req.Description)
	setOrDelete(ev, ical.PropCategories, strings.TrimSpace(req.Calendar))
	ev.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	setEventTimes(&ical.Event{Component: ev}, req)
	editRRule(ev, req.Repeat)
	editReminder(ev, req)
	editGuests(ev, req.Guests, owner)

	var buf strings.Builder
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		return "", false
	}
	return buf.String(), true
}

// setOrDelete writes a text property, or removes it when the value is empty.
func setOrDelete(ev *ical.Component, name, value string) {
	if v := strings.TrimSpace(value); v != "" {
		ev.Props.SetText(name, icalText(value))
	} else {
		ev.Props.Del(name)
	}
}

// editRRule updates the recurrence rule only when the frequency changes, so an
// externally-authored rule (BYDAY, COUNT, INTERVAL, …) survives an unrelated edit.
func editRRule(ev *ical.Component, repeat string) {
	newFreq := normalizeFreq(repeat)
	if newFreq == rruleFreq(propText(ev.Props, ical.PropRecurrenceRule)) {
		return
	}
	if newFreq == "" {
		ev.Props.Del(ical.PropRecurrenceRule)
		return
	}
	p := ical.NewProp(ical.PropRecurrenceRule)
	p.Value = "FREQ=" + newFreq
	ev.Props.Set(p)
}

// editReminder replaces only the simple display reminder the form manages,
// leaving any other VALARM (EMAIL/AUDIO or non-minute triggers) intact.
func editReminder(ev *ical.Component, req createEventRequest) {
	if req.Reminder == alarmMinutes(ev.Children) {
		return
	}
	kept := ev.Children[:0:0]
	for _, ch := range ev.Children {
		if !isSimpleReminder(ch) {
			kept = append(kept, ch)
		}
	}
	ev.Children = kept
	if req.Reminder > 0 {
		alarm := ical.NewComponent(ical.CompAlarm)
		alarm.Props.SetText(ical.PropAction, "DISPLAY")
		alarm.Props.SetText(ical.PropDescription, icalText(req.Summary))
		tp := ical.NewProp(ical.PropTrigger)
		tp.Value = fmt.Sprintf("-PT%dM", req.Reminder)
		alarm.Props.Set(tp)
		ev.Children = append(ev.Children, alarm)
	}
}

// isSimpleReminder reports whether a VALARM is the minutes-before display alarm
// the webmail form owns.
func isSimpleReminder(ch *ical.Component) bool {
	if ch.Name != ical.CompAlarm {
		return false
	}
	if a := ch.Props.Get(ical.PropAction); a == nil || !strings.EqualFold(a.Value, "DISPLAY") {
		return false
	}
	tp := ch.Props.Get(ical.PropTrigger)
	return tp != nil && triggerMinutes(tp.Value) > 0
}

// editGuests reconciles the guest ATTENDEE set to req.Guests, keeping existing
// entries (and their CN/ROLE/PARTSTAT) and the owner's ATTENDEE untouched.
func editGuests(ev *ical.Component, guests []string, owner string) {
	want := map[string]bool{}
	for _, g := range guests {
		if g = strings.TrimSpace(g); g != "" && !strings.EqualFold(g, owner) {
			want[strings.ToLower(g)] = true
		}
	}
	have := map[string]bool{}
	kept := make([]ical.Prop, 0, len(ev.Props[ical.PropAttendee]))
	for _, p := range ev.Props[ical.PropAttendee] {
		email := strings.ToLower(strings.TrimPrefix(p.Value, mailtoPrefix))
		if strings.EqualFold(email, owner) {
			kept = append(kept, p) // owner (with RSVP) stays
			continue
		}
		if want[email] {
			kept = append(kept, p) // keep guest, preserving its params
			have[email] = true
		}
	}
	ev.Props[ical.PropAttendee] = kept
	for g := range want {
		if !have[g] {
			p := ical.NewProp(ical.PropAttendee)
			p.Value = mailtoPrefix + g
			ev.Props.Add(p)
		}
	}
}

func (s *Server) handleCalendarDelete(w http.ResponseWriter, r *http.Request) {
	user := webmailCreds(r).Email
	uid := r.PathValue("uid")
	if err := s.davStore.DeleteCalObject(user, webmailCalendarID, uid); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleCalendarRsvp records the mailbox owner's response (yes/maybe/no/none) on
// an event by setting their ATTENDEE PARTSTAT, in place.
func (s *Server) handleCalendarRsvp(w http.ResponseWriter, r *http.Request) {
	user := webmailCreds(r).Email
	uid := r.PathValue("uid")
	var req struct {
		Rsvp string `json:"rsvp"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	obj, err := s.davStore.GetCalObject(user, webmailCalendarID, uid)
	if err != nil || obj == nil {
		s.writeError(w, http.StatusNotFound, errEventNotFound)
		return
	}
	partstat := rsvpToPartstat(req.Rsvp)
	data, ok := setOwnerPartstat(obj.Data, user, partstat)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, fmt.Errorf("could not update the event"))
		return
	}
	if _, err := s.davStore.PutCalObject(user, webmailCalendarID, uid, data); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"rsvp": partstatToRsvp(partstat)})
}

// handleCalendarAttachment streams a single event attachment by its position in
// the VEVENT's ATTACH list.
func (s *Server) handleCalendarAttachment(w http.ResponseWriter, r *http.Request) {
	user := webmailCreds(r).Email
	uid := r.PathValue("uid")
	idx, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || idx < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid attachment index"})
		return
	}
	obj, err := s.davStore.GetCalObject(user, webmailCalendarID, uid)
	if err != nil || obj == nil {
		s.writeError(w, http.StatusNotFound, errEventNotFound)
		return
	}
	name, mime, raw, ok := eventAttachmentAt(obj.Data, idx)
	if !ok {
		s.writeError(w, http.StatusNotFound, fmt.Errorf("attachment not found"))
		return
	}
	if mime == "" {
		mime = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+sanitizeFilename(name)+"\"")
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	_, _ = w.Write(raw)
}

// firstEvent decodes an iCalendar object and returns its single VEVENT.
func firstEvent(data string) (ical.Event, bool) {
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return ical.Event{}, false
	}
	events := cal.Events()
	if len(events) == 0 {
		return ical.Event{}, false
	}
	return events[0], true // one VEVENT per stored object
}

// parseEvent extracts the first VEVENT from an iCalendar object into the JSON
// event shape. owner is the logged-in mailbox, used to read back its RSVP and to
// keep it out of the guest list. It returns ok=false for objects it cannot read.
func parseEvent(data, owner string) (calendarEvent, bool) {
	e, ok := firstEvent(data)
	if !ok {
		return calendarEvent{}, false
	}
	ev := calendarEvent{
		UID:         propText(e.Props, ical.PropUID),
		Summary:     propTextDecoded(e.Props, ical.PropSummary),
		Location:    propTextDecoded(e.Props, ical.PropLocation),
		Description: propTextDecoded(e.Props, ical.PropDescription),
		Calendar:    propTextDecoded(e.Props, ical.PropCategories),
		AllDay:      isAllDay(e),
		Guests:      eventGuests(e, owner),
		Repeat:      rruleFreq(propText(e.Props, ical.PropRecurrenceRule)),
		Reminder:    alarmMinutes(e.Children),
		Rsvp:        ownerRsvp(e, owner),
		Attachments: eventAttachments(e),
	}
	if start, err := e.DateTimeStart(time.UTC); err == nil {
		ev.Start = start
	}
	if end, err := e.DateTimeEnd(time.UTC); err == nil {
		ev.End = end
	}
	return ev, true
}

// propText returns a property's raw value, or "" when it is absent.
func propText(props ical.Props, name string) string {
	if p := props.Get(name); p != nil {
		return p.Value
	}
	return ""
}

// propTextDecoded returns a TEXT property with iCalendar escaping (\n \, \; \\)
// unfolded, or "" when it is absent or malformed.
func propTextDecoded(props ical.Props, name string) string {
	s, _ := props.Text(name)
	return s
}

// isAllDay reports whether the event's DTSTART is a date-only value.
func isAllDay(e ical.Event) bool {
	p := e.Props.Get(ical.PropDateTimeStart)
	return p != nil && strings.EqualFold(p.Params.Get(ical.ParamValue), "DATE")
}

// eventGuests reads the mailto: attendees off an event, excluding the owner
// (whose ATTENDEE carries their own RSVP, not a guest invitation).
func eventGuests(e ical.Event, owner string) []string {
	var out []string
	for _, p := range e.Props.Values(ical.PropAttendee) {
		g := strings.TrimPrefix(p.Value, mailtoPrefix)
		if g == "" || strings.EqualFold(g, owner) {
			continue
		}
		out = append(out, g)
	}
	return out
}

// ownerRsvp reads the owner's ATTENDEE PARTSTAT as an RSVP token (yes/maybe/no),
// or "none" when they have not responded.
func ownerRsvp(e ical.Event, owner string) string {
	if owner == "" {
		return "none"
	}
	target := mailtoPrefix + owner
	for _, p := range e.Props.Values(ical.PropAttendee) {
		if strings.EqualFold(p.Value, target) {
			return partstatToRsvp(p.Params.Get(partstatParam))
		}
	}
	return "none"
}

// rsvpToPartstat maps an RSVP token to an iCalendar PARTSTAT.
func rsvpToPartstat(rsvp string) string {
	switch strings.ToLower(strings.TrimSpace(rsvp)) {
	case "yes":
		return "ACCEPTED"
	case "maybe":
		return "TENTATIVE"
	case "no":
		return "DECLINED"
	default:
		return "NEEDS-ACTION"
	}
}

// partstatToRsvp maps an iCalendar PARTSTAT back to an RSVP token.
func partstatToRsvp(ps string) string {
	switch strings.ToUpper(strings.TrimSpace(ps)) {
	case "ACCEPTED":
		return "yes"
	case "TENTATIVE":
		return "maybe"
	case "DECLINED":
		return "no"
	default:
		return "none"
	}
}

// setOwnerPartstat rewrites the stored iCalendar object with the owner's
// ATTENDEE PARTSTAT set to partstat, adding the ATTENDEE if it is missing.
func setOwnerPartstat(data, owner, partstat string) (string, bool) {
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return "", false
	}
	var ev *ical.Component
	for _, ch := range cal.Children {
		if ch.Name == ical.CompEvent {
			ev = ch
			break
		}
	}
	if ev == nil {
		return "", false
	}
	target := mailtoPrefix + owner
	found := false
	for i := range ev.Props[ical.PropAttendee] {
		if strings.EqualFold(ev.Props[ical.PropAttendee][i].Value, target) {
			ev.Props[ical.PropAttendee][i].Params.Set(partstatParam, partstat)
			found = true
			break
		}
	}
	if !found {
		p := ical.NewProp(ical.PropAttendee)
		p.Value = target
		p.Params.Set(partstatParam, partstat)
		ev.Props.Add(p)
	}
	var buf strings.Builder
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		return "", false
	}
	return buf.String(), true
}

// eventAttachments reads attachment metadata (no data) off an event.
func eventAttachments(e ical.Event) []eventAttachment {
	var out []eventAttachment
	for i, p := range e.Props.Values(ical.PropAttach) {
		att := eventAttachment{Filename: p.Params.Get(attachFilenameParam), Mime: p.Params.Get(ical.ParamFormatType)}
		if att.Filename == "" {
			att.Filename = fmt.Sprintf(attachNameFmt, i+1)
		}
		if raw, err := p.Binary(); err == nil {
			att.Size = len(raw)
		}
		out = append(out, att)
	}
	return out
}

// eventAttachmentAt returns the decoded bytes of the idx-th ATTACH in the event.
func eventAttachmentAt(data string, idx int) (name, mime string, raw []byte, ok bool) {
	e, found := firstEvent(data)
	if !found {
		return "", "", nil, false
	}
	atts := e.Props.Values(ical.PropAttach)
	if idx >= len(atts) {
		return "", "", nil, false
	}
	p := atts[idx]
	b, err := p.Binary()
	if err != nil {
		return "", "", nil, false
	}
	name = p.Params.Get(attachFilenameParam)
	if name == "" {
		name = fmt.Sprintf(attachNameFmt, idx+1)
	}
	return name, p.Params.Get(ical.ParamFormatType), b, true
}

// buildEvent renders a create request into an iCalendar VEVENT object. owner is
// the logged-in mailbox, recorded as an ATTENDEE carrying its RSVP.
func buildEvent(uid string, req createEventRequest, owner string) string {
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "-//Mailfold//Calendar//EN")

	e := ical.NewEvent()
	e.Props.SetText(ical.PropUID, uid)
	e.Props.SetText(ical.PropSummary, icalText(req.Summary))
	setEventText(e, ical.PropLocation, req.Location)
	setEventText(e, ical.PropDescription, req.Description)
	setEventText(e, ical.PropCategories, strings.TrimSpace(req.Calendar))
	e.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	setEventTimes(e, req)
	addGuests(e, req.Guests, owner)
	addOwnerAttendee(e, owner, req.Rsvp)
	if freq := normalizeFreq(req.Repeat); freq != "" {
		p := ical.NewProp(ical.PropRecurrenceRule)
		p.Value = "FREQ=" + freq
		e.Props.Set(p)
	}
	addAttachments(e, req.Attachments)
	addReminder(e, req)
	cal.Children = append(cal.Children, e.Component)

	var buf strings.Builder
	_ = ical.NewEncoder(&buf).Encode(cal)
	return buf.String()
}

// setEventText sets a text property when the value is non-empty.
func setEventText(e *ical.Event, name, value string) {
	if value != "" {
		e.Props.SetText(name, icalText(value))
	}
}

// setEventTimes writes DTSTART/DTEND as date-only or date-time values.
func setEventTimes(e *ical.Event, req createEventRequest) {
	if req.AllDay {
		setDate(e, ical.PropDateTimeStart, req.Start)
		setDate(e, ical.PropDateTimeEnd, req.End)
		return
	}
	e.Props.SetDateTime(ical.PropDateTimeStart, req.Start.UTC())
	e.Props.SetDateTime(ical.PropDateTimeEnd, req.End.UTC())
}

// addGuests appends ATTENDEE properties for each invited email, skipping the
// owner (added separately with their RSVP).
func addGuests(e *ical.Event, guests []string, owner string) {
	for _, g := range guests {
		if g = strings.TrimSpace(g); g != "" && !strings.EqualFold(g, owner) {
			p := ical.NewProp(ical.PropAttendee)
			p.Value = mailtoPrefix + g
			e.Props.Add(p)
		}
	}
}

// addOwnerAttendee records the mailbox owner as an ATTENDEE carrying their RSVP
// as PARTSTAT.
func addOwnerAttendee(e *ical.Event, owner, rsvp string) {
	if owner == "" {
		return
	}
	p := ical.NewProp(ical.PropAttendee)
	p.Value = mailtoPrefix + owner
	p.Params.Set(partstatParam, rsvpToPartstat(rsvp))
	e.Props.Add(p)
}

// addAttachments appends inline (base64) ATTACH properties.
func addAttachments(e *ical.Event, atts []eventAttachment) {
	for _, a := range atts {
		raw, err := base64.StdEncoding.DecodeString(a.Data)
		if err != nil || len(raw) == 0 {
			continue
		}
		p := ical.NewProp(ical.PropAttach)
		p.SetBinary(raw)
		if m := icalParamSafe(a.Mime); m != "" {
			p.Params.Set(ical.ParamFormatType, m)
		}
		if f := icalParamSafe(a.Filename); f != "" {
			p.Params.Set(attachFilenameParam, f)
		}
		e.Props.Add(p)
	}
}

// addReminder attaches a display VALARM that fires Reminder minutes before start.
func addReminder(e *ical.Event, req createEventRequest) {
	if req.Reminder <= 0 {
		return
	}
	alarm := ical.NewComponent(ical.CompAlarm)
	alarm.Props.SetText(ical.PropAction, "DISPLAY")
	alarm.Props.SetText(ical.PropDescription, icalText(req.Summary))
	tp := ical.NewProp(ical.PropTrigger)
	tp.Value = fmt.Sprintf("-PT%dM", req.Reminder)
	alarm.Props.Set(tp)
	e.Children = append(e.Children, alarm)
}

// icalText normalises a text value for iCalendar encoding. SetText escapes "\n"
// but leaves a bare CR, which the encoder rejects; fold CRLF/CR to LF first.
func icalText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// icalParamSafe drops characters that are illegal in an iCalendar parameter
// value (double-quote and control characters) so encoding cannot fail.
func icalParamSafe(v string) string {
	return strings.Map(func(r rune) rune {
		if r == '"' || r == '\r' || r == '\n' || r < 0x20 {
			return -1
		}
		return r
	}, v)
}

// setDate writes a date-only (all-day) property (VALUE=DATE).
func setDate(e *ical.Event, name string, t time.Time) {
	prop := ical.NewProp(name)
	prop.Params.Set(ical.ParamValue, "DATE")
	prop.Value = t.Format("20060102")
	e.Props.Set(prop)
}

// normalizeFreq maps a repeat token to a canonical iCalendar FREQ, or "" when
// the event does not repeat.
func normalizeFreq(repeat string) string {
	switch strings.ToUpper(strings.TrimSpace(repeat)) {
	case "DAILY":
		return "DAILY"
	case "WEEKLY":
		return "WEEKLY"
	case "MONTHLY":
		return "MONTHLY"
	case "YEARLY":
		return "YEARLY"
	default:
		return ""
	}
}

// rruleFreq extracts the FREQ value from an RRULE string (e.g. "FREQ=WEEKLY").
func rruleFreq(rule string) string {
	for _, part := range strings.Split(rule, ";") {
		if k, v, ok := strings.Cut(part, "="); ok && strings.EqualFold(strings.TrimSpace(k), "FREQ") {
			return strings.ToUpper(strings.TrimSpace(v))
		}
	}
	return ""
}

// alarmMinutes reads a VALARM's minutes-before-start trigger, returning 0 when
// the event has no simple reminder.
func alarmMinutes(children []*ical.Component) int {
	for _, ch := range children {
		if ch.Name != ical.CompAlarm {
			continue
		}
		if tp := ch.Props.Get(ical.PropTrigger); tp != nil {
			return triggerMinutes(tp.Value)
		}
	}
	return 0
}

// triggerMinutes parses a "-PT<n>M" duration into minutes; anything else is 0.
func triggerMinutes(v string) int {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "-PT") || !strings.HasSuffix(v, "M") {
		return 0
	}
	n, err := strconv.Atoi(v[3 : len(v)-1])
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// sanitizeFilename strips characters that would break a Content-Disposition
// header or escape the download name.
func sanitizeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r == '/' || r < 0x20 {
			return '_'
		}
		return r
	}, name)
	if name == "" {
		return "attachment"
	}
	return name
}

// newEventUID returns a random iCalendar UID.
func newEventUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b) + "@mailfold"
}
