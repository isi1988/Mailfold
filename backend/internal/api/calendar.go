package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// webmailCalendarID is the single calendar collection each mailbox's webmail
// calendar uses. It is created on demand.
const webmailCalendarID = "default"

// attachFilenameParam carries an attachment's original filename alongside the
// inline binary ATTACH property.
const attachFilenameParam = "X-FILENAME"

// maxCalAttachTotal caps the combined decoded size of an event's attachments so
// a single VEVENT cannot bloat the SQLite store.
const maxCalAttachTotal = 10 << 20 // 10 MiB

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
	mux.HandleFunc("DELETE /api/webmail/calendar/events/{uid}", s.requireWebmail(s.handleCalendarDelete))
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
		if ev, ok := parseEvent(o.Data); ok {
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
	data := buildEvent(uid, req)
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

func (s *Server) handleCalendarDelete(w http.ResponseWriter, r *http.Request) {
	user := webmailCreds(r).Email
	uid := r.PathValue("uid")
	if err := s.davStore.DeleteCalObject(user, webmailCalendarID, uid); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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
		s.writeError(w, http.StatusNotFound, fmt.Errorf("event not found"))
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
// event shape. It returns ok=false for objects it cannot read (e.g. VTODOs).
func parseEvent(data string) (calendarEvent, bool) {
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
		Guests:      eventGuests(e),
		Repeat:      rruleFreq(propText(e.Props, ical.PropRecurrenceRule)),
		Reminder:    alarmMinutes(e),
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

// eventGuests reads the mailto: attendees off an event.
func eventGuests(e ical.Event) []string {
	var out []string
	for _, p := range e.Props.Values(ical.PropAttendee) {
		if g := strings.TrimPrefix(p.Value, "mailto:"); g != "" {
			out = append(out, g)
		}
	}
	return out
}

// eventAttachments reads attachment metadata (no data) off an event.
func eventAttachments(e ical.Event) []eventAttachment {
	var out []eventAttachment
	for i, p := range e.Props.Values(ical.PropAttach) {
		att := eventAttachment{Filename: p.Params.Get(attachFilenameParam), Mime: p.Params.Get(ical.ParamFormatType)}
		if att.Filename == "" {
			att.Filename = fmt.Sprintf("attachment-%d", i+1)
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
		name = fmt.Sprintf("attachment-%d", idx+1)
	}
	return name, p.Params.Get(ical.ParamFormatType), b, true
}

// buildEvent renders a create request into an iCalendar VEVENT object.
func buildEvent(uid string, req createEventRequest) string {
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
	addGuests(e, req.Guests)
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

// addGuests appends ATTENDEE properties for each invited email.
func addGuests(e *ical.Event, guests []string) {
	for _, g := range guests {
		if g = strings.TrimSpace(g); g != "" {
			p := ical.NewProp(ical.PropAttendee)
			p.Value = "mailto:" + g
			e.Props.Add(p)
		}
	}
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
func alarmMinutes(e ical.Event) int {
	for _, ch := range e.Children {
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
