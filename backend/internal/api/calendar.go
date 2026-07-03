package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// webmailCalendarID is the single calendar collection each mailbox's webmail
// calendar uses. It is created on demand.
const webmailCalendarID = "default"

// calendarEvent is the JSON shape the webmail calendar UI works with.
type calendarEvent struct {
	UID         string    `json:"uid"`
	Summary     string    `json:"summary"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	AllDay      bool      `json:"all_day"`
	Location    string    `json:"location,omitempty"`
	Description string    `json:"description,omitempty"`
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
	Summary     string    `json:"summary"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	AllDay      bool      `json:"all_day"`
	Location    string    `json:"location"`
	Description string    `json:"description"`
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

// parseEvent extracts the first VEVENT from an iCalendar object into the JSON
// event shape. It returns ok=false for objects it cannot read (e.g. VTODOs).
func parseEvent(data string) (calendarEvent, bool) {
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return calendarEvent{}, false
	}
	events := cal.Events()
	if len(events) == 0 {
		return calendarEvent{}, false
	}
	e := events[0] // one VEVENT per stored object
	ev := calendarEvent{}
	if p := e.Props.Get(ical.PropUID); p != nil {
		ev.UID = p.Value
	}
	if p := e.Props.Get(ical.PropSummary); p != nil {
		ev.Summary = p.Value
	}
	if p := e.Props.Get(ical.PropLocation); p != nil {
		ev.Location = p.Value
	}
	if p := e.Props.Get(ical.PropDescription); p != nil {
		ev.Description = p.Value
	}
	if start, err := e.DateTimeStart(time.UTC); err == nil {
		ev.Start = start
	}
	if end, err := e.DateTimeEnd(time.UTC); err == nil {
		ev.End = end
	}
	if p := e.Props.Get(ical.PropDateTimeStart); p != nil && strings.EqualFold(p.Params.Get(ical.ParamValue), "DATE") {
		ev.AllDay = true
	}
	return ev, true
}

// buildEvent renders a create request into an iCalendar VEVENT object.
func buildEvent(uid string, req createEventRequest) string {
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "-//Mailfold//Calendar//EN")

	e := ical.NewEvent()
	e.Props.SetText(ical.PropUID, uid)
	e.Props.SetText(ical.PropSummary, req.Summary)
	if req.Location != "" {
		e.Props.SetText(ical.PropLocation, req.Location)
	}
	if req.Description != "" {
		e.Props.SetText(ical.PropDescription, req.Description)
	}
	e.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	if req.AllDay {
		setDate(e, ical.PropDateTimeStart, req.Start)
		setDate(e, ical.PropDateTimeEnd, req.End)
	} else {
		e.Props.SetDateTime(ical.PropDateTimeStart, req.Start.UTC())
		e.Props.SetDateTime(ical.PropDateTimeEnd, req.End.UTC())
	}
	cal.Children = append(cal.Children, e.Component)

	var buf strings.Builder
	_ = ical.NewEncoder(&buf).Encode(cal)
	return buf.String()
}

// setDate writes a date-only (all-day) property (VALUE=DATE).
func setDate(e *ical.Event, name string, t time.Time) {
	prop := ical.NewProp(name)
	prop.Params.Set(ical.ParamValue, "DATE")
	prop.Value = t.Format("20060102")
	e.Props.Set(prop)
}

// newEventUID returns a random iCalendar UID.
func newEventUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b) + "@mailfold"
}
