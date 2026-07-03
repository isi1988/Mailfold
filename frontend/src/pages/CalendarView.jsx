import React, { useState, useEffect, useCallback, useRef } from 'react';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Segmented } from '../ds/components/atoms/Segmented.jsx';
import { Toggle } from '../ds/components/atoms/Toggle.jsx';
import { cx } from '../ds/lib/cx.js';
import { wm } from '../api/webmail.js';
import { useWebmailAuth } from '../auth/WebmailAuthContext.jsx';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';

const WEEKDAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
const MINI_HEAD = ['M', 'T', 'W', 'T', 'F', 'S', 'S'];
const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
const MAX_ATTACH_TOTAL = 10 * 1024 * 1024; // keep in step with the backend cap
const DAY_START = 7;
const DAY_END = 21;

const pad = n => String(n).padStart(2, '0');
const ymd = d => d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate());
const sameDay = (a, b) => a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
const hhmm = iso => { const d = new Date(iso); return isNaN(d) ? '' : pad(d.getHours()) + ':' + pad(d.getMinutes()); };
const humanSize = n => (n < 1024 ? n + ' B' : n < 1024 * 1024 ? (n / 1024).toFixed(0) + ' KB' : (n / 1024 / 1024).toFixed(1) + ' MB');

// Shared field styling matching the design's modal inputs.
const FIELD = { width: '100%', padding: '10px 12px', border: '1px solid var(--hair)', borderRadius: 9, background: 'var(--surface-2)', color: 'var(--ink)', font: '400 13.5px system-ui', outline: 'none', boxSizing: 'border-box' };
const FIELD_LABEL = { fontSize: 12, fontWeight: 600, color: 'var(--ink)', marginBottom: 7, display: 'block' };
const REPEATS = [['', 'repeatNever'], ['DAILY', 'repeatDaily'], ['WEEKLY', 'repeatWeekly'], ['MONTHLY', 'repeatMonthly'], ['YEARLY', 'repeatYearly']];
const REMINDERS = [[0, 'reminderNone'], [10, 'reminder10'], [30, 'reminder30'], [60, 'reminder60'], [1440, 'reminder1440']];

// CAL_COLORS keys a calendar category to [soft background, ink, dot] tokens.
const CAL_COLORS = {
  work: ['var(--accent-soft)', 'var(--accent-ink)', 'var(--accent)'],
  personal: ['var(--green-soft)', 'var(--green)', 'var(--green)'],
  team: ['var(--blue-soft)', 'var(--blue)', 'var(--blue)'],
  holiday: ['var(--amber-soft)', 'var(--amber)', 'var(--amber)'],
  holidays: ['var(--amber-soft)', 'var(--amber)', 'var(--amber)'],
};
const CAL_LIST = [['work', 'Work'], ['personal', 'Personal'], ['team', 'Team'], ['holidays', 'Holidays']];
const calKey = cal => (cal || 'work').toLowerCase();
const calColors = cal => CAL_COLORS[calKey(cal)] || CAL_COLORS.work;
const calName = cal => ({ work: 'Work', personal: 'Personal', team: 'Team', holiday: 'Holidays', holidays: 'Holidays' }[calKey(cal)] || cal || 'Work');
const RSVP_OPTS = [['yes', 'rsvpGoing'], ['maybe', 'rsvpMaybe'], ['no', 'rsvpCant']];

// videoLink finds a joinable meeting URL in an event's location or description.
const VIDEO_RE = /(https?:\/\/[^\s]*(?:zoom\.us|meet\.google\.com|teams\.microsoft\.com|meet\.jit\.si|whereby\.com|webex\.com|around\.co)[^\s]*)/i;
function videoLink(ev) {
  const m = ((ev.location || '') + ' ' + (ev.description || '')).match(VIDEO_RE);
  if (m) return m[1];
  const loc = (ev.location || '').trim();
  return /^https?:\/\/\S+$/i.test(loc) ? loc : null;
}

// eventToReq turns a listed event into a create/update request body.
const eventToReq = ev => ({
  summary: ev.summary, all_day: !!ev.all_day, calendar: ev.calendar || '', location: ev.location || '',
  description: ev.description || '', guests: ev.guests || [], repeat: ev.repeat || '', reminder: ev.reminder || 0,
  start: ev.start, end: ev.end,
});

// readFileAsBase64 resolves to the file's base64 payload (without the data: prefix).
const readFileAsBase64 = file => new Promise((resolve, reject) => {
  const r = new FileReader();
  r.onerror = () => reject(new Error('read failed'));
  r.onload = () => resolve(String(r.result).split(',')[1] || '');
  r.readAsDataURL(file);
});

// ---- date helpers ------------------------------------------------------------

function monthGrid(year, month) {
  const first = new Date(year, month, 1);
  const offset = (first.getDay() + 6) % 7; // Monday = 0
  const start = new Date(year, month, 1 - offset);
  const weeks = [];
  for (let w = 0; w < 6; w++) {
    const days = [];
    for (let d = 0; d < 7; d++) days.push(new Date(start.getFullYear(), start.getMonth(), start.getDate() + w * 7 + d));
    weeks.push(days);
  }
  return weeks;
}
function startOfWeek(d) {
  const x = new Date(d.getFullYear(), d.getMonth(), d.getDate());
  x.setDate(x.getDate() - ((x.getDay() + 6) % 7));
  return x;
}
function weekDays(d) {
  const s = startOfWeek(d);
  return Array.from({ length: 7 }, (_, i) => new Date(s.getFullYear(), s.getMonth(), s.getDate() + i));
}
function weekRangeLabel(d) {
  const days = weekDays(d), a = days[0], b = days[6], mo = m => MONTHS[m].slice(0, 3);
  return a.getMonth() === b.getMonth()
    ? `${mo(a.getMonth())} ${a.getDate()} – ${b.getDate()}, ${b.getFullYear()}`
    : `${mo(a.getMonth())} ${a.getDate()} – ${mo(b.getMonth())} ${b.getDate()}`;
}
// All-day events are stored at UTC midnight; read their calendar day in UTC so
// they do not shift a day in non-UTC time zones.
const allDayToLocal = iso => { const d = new Date(iso); return new Date(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate()); };
const eventStartDate = ev => (ev && ev.all_day ? allDayToLocal(ev.start) : new Date(ev.start));
const eventEndDate = ev => (ev && ev.all_day ? allDayToLocal(ev.end) : new Date(ev.end));
const dayOf = d => new Date(d.getFullYear(), d.getMonth(), d.getDate());
const addDays = (d, n) => { const x = new Date(d); x.setDate(x.getDate() + n); return x; };

// ---- recurrence + multi-day --------------------------------------------------

const WEEKDAY_CODE = { SU: 0, MO: 1, TU: 2, WE: 3, TH: 4, FR: 5, SA: 6 };

function parseICalDate(v) {
  const m = (v || '').match(/^(\d{4})(\d{2})(\d{2})/);
  return m ? new Date(Date.UTC(+m[1], +m[2] - 1, +m[3], 23, 59, 59)) : null;
}
function parseRRule(rrule) {
  const p = { interval: 1, byday: [] };
  for (const part of (rrule || '').split(';')) {
    const [k, v] = part.split('=');
    const key = (k || '').toUpperCase();
    if (key === 'FREQ') p.freq = (v || '').toUpperCase();
    else if (key === 'INTERVAL') p.interval = Math.max(1, parseInt(v, 10) || 1);
    else if (key === 'COUNT') p.count = parseInt(v, 10) || 0;
    else if (key === 'UNTIL') p.until = parseICalDate(v);
    else if (key === 'BYDAY') p.byday = (v || '').split(',').map(x => WEEKDAY_CODE[x.trim().slice(-2).toUpperCase()]).filter(n => n != null);
  }
  return p;
}
const stepFreq = (d, freq, n) => {
  const x = new Date(d);
  if (freq === 'DAILY') x.setDate(x.getDate() + n);
  else if (freq === 'WEEKLY') x.setDate(x.getDate() + 7 * n);
  else if (freq === 'MONTHLY') x.setMonth(x.getMonth() + n);
  else if (freq === 'YEARLY') x.setFullYear(x.getFullYear() + n);
  return x;
};

// occurrenceStarts returns the start Dates of an event's occurrences that could
// be visible in [rangeStart, rangeEnd]. Bounded so a malformed rule can't loop.
function occurrenceStarts(ev, rangeStart, rangeEnd) {
  const base = new Date(ev.start);
  const r = ev.rrule && /FREQ=/i.test(ev.rrule) ? parseRRule(ev.rrule) : null;
  if (!r || !r.freq) return [base];
  const out = [];
  let n = 0;
  if (r.freq === 'WEEKLY' && r.byday.length) {
    const week0 = startOfWeek(base);
    for (let i = 0; i < 1500; i++) {
      const wk = stepFreq(week0, 'WEEKLY', i * r.interval);
      if (wk > rangeEnd) break;
      for (const wd of r.byday.slice().sort((a, b) => a - b)) {
        const occ = addDays(wk, (wd + 6) % 7); // Monday-first column offset
        occ.setHours(base.getHours(), base.getMinutes(), 0, 0);
        if (occ < base) continue;
        if (r.until && occ > r.until) return out;
        if (r.count && n >= r.count) return out;
        n++;
        if (occ >= rangeStart && occ <= rangeEnd) out.push(occ);
      }
    }
    return out;
  }
  for (let i = 0; i < 1500; i++) {
    const occ = stepFreq(base, r.freq, i * r.interval);
    if (occ > rangeEnd) break;
    if (r.until && occ > r.until) break;
    if (r.count && n >= r.count) break;
    n++;
    if (occ >= rangeStart) out.push(occ);
  }
  return out;
}

// expandInstances turns stored events into display instances: one per recurrence
// occurrence, carrying the original event as `_orig`.
function expandInstances(events, rangeStart, rangeEnd) {
  const out = [];
  for (const ev of events) {
    const durMs = Math.max(0, new Date(ev.end) - new Date(ev.start));
    for (const s of occurrenceStarts(ev, rangeStart, rangeEnd)) {
      const end = new Date(s.getTime() + durMs);
      out.push({ ...ev, start: s.toISOString(), end: end.toISOString(), _orig: ev, _series: !!(ev.rrule && /FREQ=/i.test(ev.rrule)), _key: ev.uid + '@' + s.getTime() });
    }
  }
  return out;
}

// instLastDay is the last calendar day an instance covers (all-day DTEND is exclusive).
function instLastDay(inst) {
  if (inst.all_day) { const e = addDays(allDayToLocal(inst.end), -1); const f = dayOf(eventStartDate(inst)); return e < f ? f : e; }
  return dayOf(new Date(inst.end));
}
const coversDay = (inst, day) => { const d = dayOf(day); return d >= dayOf(eventStartDate(inst)) && d <= instLastDay(inst); };
const eventsOn = (instances, day) => instances.filter(i => coversDay(i, day)).sort((a, b) => eventStartDate(a) - eventStartDate(b));

// ---- event pill --------------------------------------------------------------

function RsvpMark({ rsvp }) {
  if (rsvp === 'yes') return <span className="mf-rsvp mf-rsvp--yes" title="Going"><svg width="8" height="8" viewBox="0 0 12 12" fill="none"><path d="M2.4 6.4l2.2 2.2 5-5.2" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" /></svg></span>;
  if (rsvp === 'maybe') return <span className="mf-rsvp mf-rsvp--maybe" title="Maybe">?</span>;
  if (rsvp === 'no') return <span className="mf-rsvp mf-rsvp--no" title="Not going"><svg width="8" height="8" viewBox="0 0 12 12" fill="none"><path d="M3 3l6 6M9 3l-6 6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" /></svg></span>;
  return null;
}

// EventPill renders a calendar event in month ('sm'), week ('md') or day ('block')
// form, coloured by category and marked by RSVP.
function EventPill({ ev, size = 'sm', onOpen, drag }) {
  const t = useT();
  const [bg, ink] = calColors(ev.calendar);
  const dot = calColors(ev.calendar)[2];
  const rsvp = ev.rsvp || 'none';
  const mods = cx(rsvp === 'maybe' && 'mf-ev--maybe', rsvp === 'no' && 'mf-ev--no');
  const dragProps = drag ? { draggable: true, onDragStart: e => drag.start(e, ev), onDragEnd: drag.end } : {};
  const click = e => { e.stopPropagation(); onOpen(ev); };
  const time = ev.all_day ? t('calendar.allDay') : hhmm(ev.start);

  if (size === 'block') {
    return (
      <div className={cx('mf-evblock', mods)} style={{ backgroundColor: bg, color: ink, cursor: 'pointer' }} onClick={click} {...dragProps}>
        <span className="mf-evblock__time">{time}</span>
        <span className="mf-evblock__title"><RsvpMark rsvp={rsvp} />{ev.summary}</span>
      </div>
    );
  }
  return (
    <div className={cx('mf-evpill', size === 'md' && 'mf-evpill--md', mods)} style={{ backgroundColor: bg, color: ink, cursor: 'pointer' }} onClick={click} {...dragProps}>
      {rsvp === 'none' ? <span className="mf-evpill__dot" style={{ background: dot }} /> : <RsvpMark rsvp={rsvp} />}
      <span className="mf-evpill__label">{ev.all_day ? ev.summary : <>{hhmm(ev.start)}&nbsp;&nbsp;{ev.summary}</>}</span>
    </div>
  );
}

// ---- new / edit event modal --------------------------------------------------

function GuestsInput({ value, onChange, placeholder }) {
  const [text, setText] = useState('');
  const add = () => {
    const v = text.trim().replace(/,$/, '').trim();
    if (v && !value.includes(v)) onChange([...value, v]);
    setText('');
  };
  const key = e => {
    if (e.key === 'Enter' || e.key === ',') { e.preventDefault(); add(); }
    else if (e.key === 'Backspace' && !text && value.length) onChange(value.slice(0, -1));
  };
  return (
    <div className="mf-multiselect" style={{ display: 'flex', flexWrap: 'wrap', gap: 6, alignItems: 'center', ...FIELD, padding: '7px 9px' }}>
      {value.map(g => (
        <span key={g} style={{ display: 'inline-flex', alignItems: 'center', gap: 5, padding: '3px 8px', borderRadius: 6, background: 'var(--accent-soft)', color: 'var(--accent-ink)', fontSize: 12.5 }}>
          {g}<span onClick={() => onChange(value.filter(x => x !== g))} style={{ cursor: 'pointer', opacity: 0.7, fontWeight: 700 }}>×</span>
        </span>
      ))}
      <input value={text} onChange={e => setText(e.target.value)} onKeyDown={key} onBlur={add} placeholder={value.length ? '' : placeholder}
        style={{ flex: 1, minWidth: 120, border: 'none', outline: 'none', background: 'transparent', color: 'var(--ink)', font: '400 13.5px system-ui' }} />
    </div>
  );
}

function EventModal({ date, event, onClose, onSaved }) {
  const t = useT();
  const { toast } = useToast();
  const editing = !!event;
  const base = event ? eventStartDate(event) : date;
  const [summary, setSummary] = useState(event ? event.summary : '');
  const [allDay, setAllDay] = useState(event ? !!event.all_day : false);
  const [startDate, setStartDate] = useState(ymd(base));
  const [startTime, setStartTime] = useState(event ? hhmm(event.start) : '09:30');
  const [endDate, setEndDate] = useState(ymd(event && event.end ? eventEndDate(event) : base));
  const [endTime, setEndTime] = useState(event ? hhmm(event.end || event.start) : '10:30');
  const [calendar, setCalendar] = useState(event ? calName(event.calendar) : 'Work');
  const [location, setLocation] = useState(event ? event.location || '' : '');
  const [guests, setGuests] = useState(event ? event.guests || [] : []);
  const [description, setDescription] = useState(event ? event.description || '' : '');
  const [attachments, setAttachments] = useState([]);
  const [keptExisting, setKeptExisting] = useState(() => (event && event.attachments ? event.attachments.map((_, i) => i) : []));
  const [repeat, setRepeat] = useState(event ? event.repeat || '' : '');
  const [reminder, setReminder] = useState(event ? event.reminder || 0 : 10);
  const [busy, setBusy] = useState(false);

  async function pickFiles(e) {
    const files = Array.from(e.target.files || []);
    e.target.value = '';
    let total = attachments.reduce((n, a) => n + a.size, 0);
    const next = [];
    for (const f of files) {
      total += f.size;
      if (total > MAX_ATTACH_TOTAL) { toast(t('calendar.tooLarge')); break; }
      next.push({ filename: f.name, mime: f.type, size: f.size, data: await readFileAsBase64(f) });
    }
    if (next.length) setAttachments(a => [...a, ...next]);
  }

  async function save() {
    if (busy) return;
    if (!summary.trim()) { toast(t('calendar.needTitle')); return; }
    setBusy(true);
    try {
      let start, end;
      if (allDay) {
        // Date-only strings parse as UTC midnight, so the picked day survives the
        // toISOString() -> backend round trip regardless of the user's time zone.
        start = new Date(startDate);
        end = new Date(endDate);
        if (!(end > start)) end = new Date(start.getTime() + 24 * 60 * 60 * 1000);
      } else {
        start = new Date(startDate + 'T' + startTime);
        end = new Date(endDate + 'T' + endTime);
        if (!(end > start)) end = new Date(start.getTime() + 60 * 60 * 1000);
      }
      const payload = {
        summary: summary.trim(), start: start.toISOString(), end: end.toISOString(),
        all_day: allDay, calendar, location: location.trim(), guests, description, repeat, reminder,
      };
      const files = attachments.map(a => ({ filename: a.filename, mime: a.mime, data: a.data }));
      if (editing) await wm.calendar.update(event.uid, { ...payload, keep_attachments: keptExisting, attachments: files });
      else await wm.calendar.create({ ...payload, attachments: files });
      toast(editing ? t('calendar.saved') : t('calendar.created'));
      onSaved();
      onClose();
    } catch (e) {
      toast(t('calendar.saveFailed'), (e && e.body && e.body.error) || (e && e.message) || '');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mf-overlay mf-overlay--center" onClick={onClose}>
      <div onClick={e => e.stopPropagation()} style={{ width: 'min(720px, 94vw)', maxHeight: '92vh', display: 'flex', flexDirection: 'column', background: 'var(--surface)', border: '1px solid var(--hair)', borderRadius: 16, boxShadow: '0 34px 90px rgba(0,0,0,.34)', overflow: 'hidden' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '16px 24px', borderBottom: '1px solid var(--hair-soft)', background: 'var(--surface-2)', flex: 'none' }}>
          <div style={{ width: 32, height: 32, borderRadius: 9, background: 'var(--accent-soft)', display: 'flex', alignItems: 'center', justifyContent: 'center', flex: 'none' }}>
            <svg width="17" height="17" viewBox="0 0 20 20" fill="none"><rect x="3" y="4.5" width="14" height="12" rx="2" stroke="var(--accent-ink)" strokeWidth="1.4" /><path d="M3 8.2h14M7 3v3M13 3v3" stroke="var(--accent-ink)" strokeWidth="1.4" strokeLinecap="round" /></svg>
          </div>
          <div style={{ fontFamily: 'var(--font-serif)', fontSize: 21, fontWeight: 600, color: 'var(--ink-strong)' }}>{editing ? t('calendar.editEvent') : t('calendar.newEvent')}</div>
          <div onClick={onClose} style={{ marginLeft: 'auto', cursor: 'pointer', color: 'var(--faint)', display: 'flex', alignItems: 'center', gap: 6, padding: '7px 12px', borderRadius: 9, font: '500 12.5px system-ui' }}>
            <svg width="16" height="16" viewBox="0 0 20 20" fill="none"><path d="M5 5l10 10M15 5L5 15" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" /></svg>{t('common.close')}
          </div>
        </div>

        <div style={{ padding: '22px 24px', display: 'flex', flexDirection: 'column', gap: 16, overflowY: 'auto' }}>
          <input placeholder={t('calendar.titlePlaceholder')} value={summary} onChange={e => setSummary(e.target.value)} autoFocus
            style={{ width: '100%', border: 'none', borderBottom: '2px solid var(--hair)', background: 'transparent', outline: 'none', font: '600 20px var(--font-serif)', color: 'var(--ink-strong)', padding: '6px 2px', boxSizing: 'border-box' }} />

          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '2px 0' }}>
            <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>{t('calendar.allDay')}</div>
            <Toggle on={allDay} onClick={() => setAllDay(v => !v)} style={{ cursor: 'pointer' }} />
          </div>

          <div style={{ display: 'flex', gap: 12 }}>
            <div style={{ flex: 1 }}><label style={FIELD_LABEL}>{t('calendar.starts')}</label><input type="date" value={startDate} onChange={e => setStartDate(e.target.value)} style={FIELD} /></div>
            <div style={{ width: 120 }}><label style={FIELD_LABEL}>{t('calendar.time')}</label><input type="time" value={startTime} onChange={e => setStartTime(e.target.value)} disabled={allDay} style={{ ...FIELD, fontFamily: 'var(--font-mono)', opacity: allDay ? 0.5 : 1 }} /></div>
          </div>
          <div style={{ display: 'flex', gap: 12 }}>
            <div style={{ flex: 1 }}><label style={FIELD_LABEL}>{t('calendar.ends')}</label><input type="date" value={endDate} onChange={e => setEndDate(e.target.value)} style={FIELD} /></div>
            <div style={{ width: 120 }}><label style={FIELD_LABEL}>{t('calendar.time')}</label><input type="time" value={endTime} onChange={e => setEndTime(e.target.value)} disabled={allDay} style={{ ...FIELD, fontFamily: 'var(--font-mono)', opacity: allDay ? 0.5 : 1 }} /></div>
          </div>

          <div><label style={FIELD_LABEL}>{t('calendar.calendar')}</label>
            <select className="mf-input" value={calendar} onChange={e => setCalendar(e.target.value)}>
              <option value="Work">Work</option><option value="Personal">Personal</option><option value="Team">Team</option><option value="Holidays">Holidays</option>
            </select>
          </div>
          <div><label style={FIELD_LABEL}>{t('calendar.location')}</label><input placeholder={t('calendar.locationPlaceholder')} value={location} onChange={e => setLocation(e.target.value)} style={FIELD} /></div>
          <div><label style={FIELD_LABEL}>{t('calendar.guests')}</label><GuestsInput value={guests} onChange={setGuests} placeholder={t('calendar.guestsPlaceholder')} /></div>
          <div><label style={FIELD_LABEL}>{t('calendar.description')}</label><textarea placeholder={t('calendar.descriptionPlaceholder')} value={description} onChange={e => setDescription(e.target.value)} style={{ ...FIELD, minHeight: 90, lineHeight: 1.6, resize: 'vertical' }} /></div>

          <div>
              <label style={FIELD_LABEL}>{t('calendar.attachments')}</label>
              {editing && (event.attachments || []).map((a, i) => keptExisting.includes(i) && (
                <div key={'e' + i} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 11px', border: '1px solid var(--hair)', borderRadius: 9, background: 'var(--surface-2)', marginBottom: 6 }}>
                  <svg width="15" height="15" viewBox="0 0 20 20" fill="none" style={{ flex: 'none', color: 'var(--faint)' }}><path d="M8 3.5v9a3 3 0 006 0V5a2 2 0 10-4 0v7.5a1 1 0 002 0V6" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" /></svg>
                  <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 13, color: 'var(--ink)' }}>{a.filename}</span>
                  <span className="mf-u-mono" style={{ fontSize: 11.5, color: 'var(--faint)' }}>{humanSize(a.size || 0)}</span>
                  <span onClick={() => setKeptExisting(k => k.filter(x => x !== i))} style={{ cursor: 'pointer', color: 'var(--faint)', fontWeight: 700, lineHeight: 1 }}>×</span>
                </div>
              ))}
              {attachments.length > 0 && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 6, marginBottom: 8 }}>
                  {attachments.map((a, i) => (
                    <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 11px', border: '1px solid var(--hair)', borderRadius: 9, background: 'var(--surface-2)' }}>
                      <svg width="15" height="15" viewBox="0 0 20 20" fill="none" style={{ flex: 'none', color: 'var(--faint)' }}><path d="M8 3.5v9a3 3 0 006 0V5a2 2 0 10-4 0v7.5a1 1 0 002 0V6" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" /></svg>
                      <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 13, color: 'var(--ink)' }}>{a.filename}</span>
                      <span className="mf-u-mono" style={{ fontSize: 11.5, color: 'var(--faint)' }}>{humanSize(a.size)}</span>
                      <span onClick={() => setAttachments(list => list.filter((_, j) => j !== i))} style={{ cursor: 'pointer', color: 'var(--faint)', fontWeight: 700, lineHeight: 1 }}>×</span>
                    </div>
                  ))}
                </div>
              )}
              <label style={{ display: 'inline-flex', alignItems: 'center', gap: 8, padding: '9px 14px', border: '1px dashed var(--hair)', borderRadius: 9, cursor: 'pointer', color: 'var(--muted)', font: '500 13px system-ui', background: 'var(--surface-2)' }}>
                <svg width="15" height="15" viewBox="0 0 20 20" fill="none"><path d="M10 4v12M4 10h12" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" /></svg>
                {t('calendar.attachFiles')}<input type="file" multiple onChange={pickFiles} style={{ display: 'none' }} />
              </label>
          </div>

          <div style={{ display: 'flex', gap: 12 }}>
            <div style={{ flex: 1 }}><label style={FIELD_LABEL}>{t('calendar.repeat')}</label>
              <select className="mf-input" value={repeat} onChange={e => setRepeat(e.target.value)}>{REPEATS.map(([v, k]) => <option key={k} value={v}>{t('calendar.' + k)}</option>)}</select>
            </div>
            <div style={{ flex: 1 }}><label style={FIELD_LABEL}>{t('calendar.reminder')}</label>
              <select className="mf-input" value={reminder} onChange={e => setReminder(Number(e.target.value))}>{REMINDERS.map(([v, k]) => <option key={k} value={v}>{t('calendar.' + k)}</option>)}</select>
            </div>
          </div>
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10, padding: '14px 24px', borderTop: '1px solid var(--hair-soft)', background: 'var(--surface-2)', flex: 'none' }}>
          <Button variant="secondary" onClick={onClose}>{t('common.cancel')}</Button>
          <Button variant="primary" onClick={save} disabled={busy}>{busy ? t('common.saving') : (editing ? t('calendar.saveChanges') : t('calendar.save'))}</Button>
        </div>
      </div>
    </div>
  );
}

// ---- event detail ------------------------------------------------------------

function RsvpButton({ kind, active, label, onClick }) {
  return <button onClick={onClick} className={cx('mf-rsvpbtn', active && 'mf-rsvpbtn--' + kind)} style={{ flex: 'none', padding: '6px 14px' }}>{label}</button>;
}
function MetaRow({ label, value }) {
  return (
    <div style={{ display: 'flex', gap: 12, fontSize: 13 }}>
      <span style={{ flex: 'none', width: 74, color: 'var(--faint)', fontWeight: 600 }}>{label}</span>
      <span style={{ flex: 1, minWidth: 0, color: 'var(--ink)', whiteSpace: 'pre-wrap' }}>{value}</span>
    </div>
  );
}
function EventDetail({ ev, onClose, onChanged, onDeleted, onEdit }) {
  const t = useT();
  const { toast } = useToast();
  const [rsvp, setRsvp] = useState(ev.rsvp || 'none');
  const [busy, setBusy] = useState(false);
  const [bg, ink] = calColors(ev.calendar);
  const dot = calColors(ev.calendar)[2];

  async function choose(v) {
    const prev = rsvp;
    setRsvp(v);
    try { await wm.calendar.setRsvp(ev.uid, v); toast(t('calendar.responseUpdated')); if (onChanged) onChanged(); }
    catch (e) { setRsvp(prev); toast(t('calendar.saveFailed'), (e && e.message) || ''); }
  }
  async function del() {
    if (busy) return;
    if (!window.confirm(t('calendar.deleteConfirm', { title: ev.summary }))) return;
    setBusy(true);
    try { await wm.calendar.del(ev.uid); toast(t('calendar.deleted')); if (onDeleted) onDeleted(); onClose(); }
    catch (e) { toast(t('calendar.saveFailed'), (e && e.message) || ''); setBusy(false); }
  }

  const call = videoLink(ev);
  const dateLabel = new Date(ev.start).toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric', ...(ev.all_day ? { timeZone: 'UTC' } : {}) });
  const timeLabel = ev.all_day ? t('calendar.allDay') : hhmm(ev.start) + (ev.end ? ' – ' + hhmm(ev.end) : '');
  const repeatKey = ev.repeat && 'repeat' + ev.repeat.charAt(0) + ev.repeat.slice(1).toLowerCase();

  return (
    <div className="mf-overlay mf-overlay--center" onClick={onClose}>
      <div onClick={e => e.stopPropagation()} style={{ width: 'min(520px, 94vw)', maxHeight: '92vh', display: 'flex', flexDirection: 'column', background: 'var(--surface)', border: '1px solid var(--hair)', borderRadius: 16, boxShadow: '0 34px 90px rgba(0,0,0,.34)', overflow: 'hidden' }}>
        <div style={{ height: 6, background: dot, flex: 'none' }} />
        <div style={{ padding: '20px 24px', display: 'flex', flexDirection: 'column', gap: 14, overflowY: 'auto' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: 7, font: '600 12px system-ui', padding: '5px 12px', borderRadius: 999, background: bg, color: ink }}><span style={{ width: 9, height: 9, borderRadius: '50%', background: dot }} />{calName(ev.calendar)}</span>
            <div onClick={onClose} style={{ marginLeft: 'auto', cursor: 'pointer', color: 'var(--faint)', display: 'flex' }}><svg width="18" height="18" viewBox="0 0 20 20" fill="none"><path d="M5 5l10 10M15 5L5 15" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" /></svg></div>
          </div>
          <div style={{ fontFamily: 'var(--font-serif)', fontSize: 23, fontWeight: 600, color: 'var(--ink-strong)' }}>{ev.summary}</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 9 }}>
            <MetaRow label={t('calendar.when')} value={dateLabel + ' · ' + timeLabel} />
            {ev.location && <MetaRow label={t('calendar.location')} value={ev.location} />}
            {ev.guests && ev.guests.length > 0 && <MetaRow label={t('calendar.guests')} value={ev.guests.join(', ')} />}
            {repeatKey && <MetaRow label={t('calendar.repeat')} value={t('calendar.' + repeatKey)} />}
          </div>
          {ev.description && <div style={{ fontSize: 13.5, lineHeight: 1.6, color: 'var(--ink)', whiteSpace: 'pre-wrap' }}>{ev.description}</div>}
          {ev.attachments && ev.attachments.length > 0 && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              {ev.attachments.map((a, i) => (
                <div key={i} onClick={() => wm.calendar.downloadAttachment(ev.uid, i, a.filename)} style={{ display: 'flex', alignItems: 'center', gap: 9, padding: '8px 11px', border: '1px solid var(--hair)', borderRadius: 9, background: 'var(--surface-2)', cursor: 'pointer' }}>
                  <svg width="15" height="15" viewBox="0 0 20 20" fill="none" style={{ flex: 'none', color: 'var(--faint)' }}><path d="M8 3.5v9a3 3 0 006 0V5a2 2 0 10-4 0v7.5a1 1 0 002 0V6" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" /></svg>
                  <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 13, color: 'var(--ink)' }}>{a.filename}</span>
                  <span className="mf-u-mono" style={{ fontSize: 11.5, color: 'var(--faint)' }}>{humanSize(a.size || 0)}</span>
                </div>
              ))}
            </div>
          )}
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 4, paddingTop: 16, borderTop: '1px solid var(--hair-soft)' }}>
            <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>{t('calendar.yourResponse')}</span>
            <div style={{ display: 'flex', gap: 8, marginLeft: 'auto' }}>{RSVP_OPTS.map(([v, k]) => <RsvpButton key={v} kind={v} active={rsvp === v} label={t('calendar.' + k)} onClick={() => choose(v)} />)}</div>
          </div>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '14px 24px', borderTop: '1px solid var(--hair-soft)', background: 'var(--surface-2)', flex: 'none' }}>
          <Button variant="danger" onClick={del} disabled={busy}>{t('common.delete')}</Button>
          <div style={{ flex: 1 }} />
          {call && <Button variant="primary" onClick={() => window.open(call, '_blank', 'noopener,noreferrer')}>{t('calendar.join')}</Button>}
          <Button variant="secondary" onClick={onEdit}>{t('calendar.edit')}</Button>
          <Button variant="secondary" onClick={onClose}>{t('common.close')}</Button>
        </div>
      </div>
    </div>
  );
}

// ---- views -------------------------------------------------------------------

function MonthView({ cursor, events, onOpen, onNew, drag }) {
  const t = useT();
  const now = new Date();
  const year = cursor.getFullYear(), month = cursor.getMonth();
  const weeks = monthGrid(year, month);
  return (
    <div className="mf-cal-card">
      <div className="mf-month__head">{WEEKDAYS.map(d => <div key={d} className="mf-month__dow">{d}</div>)}</div>
      <div className="mf-month__grid">
        {weeks.flat().map((day, i) => {
          const inMonth = day.getMonth() === month;
          const weekend = (day.getDay() + 6) % 7 >= 5;
          const dayEvents = eventsOn(events, day);
          const over = drag.overKey === ymd(day);
          return (
            <div key={i} onClick={() => onNew(day)}
              onDragOver={e => drag.over(e, ymd(day))}
              onDrop={e => drag.dropDay(e, day)}
              className={cx('mf-month__cell', !inMonth && 'mf-month__cell--out', weekend && inMonth && 'mf-month__cell--wknd')}
              style={{ cursor: 'pointer', boxShadow: over ? 'inset 0 0 0 2px var(--accent)' : 'none' }}>
              <div className={cx('mf-month__num', sameDay(day, now) && 'mf-month__num--today', !inMonth && 'mf-month__num--muted')}>{day.getDate()}</div>
              {dayEvents.slice(0, 3).map(ev => <EventPill key={ev._key || ev.uid} ev={ev} size="sm" onOpen={onOpen} drag={ev._series ? null : drag} />)}
              {dayEvents.length > 3 && <div className="mf-month__more">{t('calendar.moreCount', { n: dayEvents.length - 3 })}</div>}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function WeekView({ cursor, events, onOpen, drag }) {
  const now = new Date();
  const days = weekDays(cursor);
  const showNow = days.some(d => sameDay(d, now));
  const nowTop = ((now.getHours() * 60 + now.getMinutes()) / (24 * 60)) * 100;
  const todayCol = days.findIndex(d => sameDay(d, now));
  return (
    <div className="mf-cal-card">
      <div className="mf-week__head">
        {days.map((d, i) => {
          const today = sameDay(d, now);
          return (
            <div key={i} className={cx('mf-week__hcell', today && 'mf-week__hcell--today')}>
              <div className={cx('mf-week__name', today && 'mf-week__name--today')}>{WEEKDAYS[i]}</div>
              <div className={cx('mf-week__num', today && 'mf-week__num--today')}>{d.getDate()}</div>
            </div>
          );
        })}
      </div>
      <div className="mf-week__body">
        {showNow && (
          <div className="mf-nowline mf-nowline--week" style={{ top: nowTop + '%' }}>
            <span className="mf-nowline__label">{pad(now.getHours()) + ':' + pad(now.getMinutes())}</span>
            <span className="mf-nowline__dot" style={{ left: ((todayCol + 0.5) / 7) * 100 + '%' }} />
          </div>
        )}
        {days.map((d, i) => (
          <div key={i} className={cx('mf-week__col', sameDay(d, now) && 'mf-week__col--today')}
            onDragOver={e => drag.over(e, ymd(d))}
            onDrop={e => drag.dropDay(e, d)}
            style={{ boxShadow: drag.overKey === ymd(d) ? 'inset 0 0 0 2px var(--accent)' : 'none' }}>
            {eventsOn(events, d).map(ev => <EventPill key={ev._key || ev.uid} ev={ev} size="md" onOpen={onOpen} drag={ev._series ? null : drag} />)}
          </div>
        ))}
      </div>
    </div>
  );
}

function DayView({ cursor, events, onOpen, drag }) {
  const t = useT();
  const now = new Date();
  const isToday = sameDay(cursor, now);
  const dayEvents = eventsOn(events, cursor);
  const inWindow = ev => { const h = new Date(ev.start).getHours(); return h >= DAY_START && h <= DAY_END; };
  // All-day events and anything outside the 07:00–21:00 window sit above the
  // timeline (each keeps its real time) rather than being clamped to a boundary hour.
  const otherEvents = dayEvents.filter(ev => ev.all_day || !inWindow(ev));
  const rows = [];
  for (let h = DAY_START; h <= DAY_END; h++) {
    rows.push({ h, events: dayEvents.filter(ev => !ev.all_day && inWindow(ev) && new Date(ev.start).getHours() === h) });
  }
  return (
    <div className="mf-cal-card">
      <div className="mf-day__head">
        <span className="mf-day__title">{cursor.toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric' })}</span>
        <span className="mf-day__count">{t('calendar.eventCount', { n: dayEvents.length })}</span>
      </div>
      <div className="mf-day__body">
        <div className="mf-day__hint">{t('calendar.dragHint')}</div>
        {otherEvents.map(ev => <div key={ev._key || ev.uid} style={{ marginBottom: 6 }}><EventPill ev={ev} size="md" onOpen={onOpen} drag={ev._series ? null : drag} /></div>)}
        {rows.map(({ h, events: evs }) => (
          <div key={h} className="mf-day__row"
            onDragOver={e => drag.over(e, 'h' + h)}
            onDrop={e => drag.dropHour(e, h)}
            style={{ background: drag.overKey === 'h' + h ? 'color-mix(in srgb, var(--accent-soft) 45%, transparent)' : undefined }}>
            {isToday && now.getHours() === h && (
              <div className="mf-nowline mf-nowline--day" style={{ top: (now.getMinutes() / 60) * 100 + '%' }}>
                <span className="mf-nowline__t">{pad(now.getHours()) + ':' + pad(now.getMinutes())}</span>
                <span className="mf-nowline__dot2" /><span className="mf-nowline__rule" />
              </div>
            )}
            <div className="mf-day__gutter">{pad(h)}:00</div>
            <div className="mf-day__lane">{evs.map(ev => <EventPill key={ev._key || ev.uid} ev={ev} size="block" onOpen={onOpen} drag={ev._series ? null : drag} />)}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

// ---- sidebar -----------------------------------------------------------------

function MiniMonth({ cursor, onPick }) {
  const now = new Date();
  const cells = monthGrid(cursor.getFullYear(), cursor.getMonth()).flat();
  return (
    <div className="mf-mini">
      <div className="mf-mini__label">{MONTHS[cursor.getMonth()]} {cursor.getFullYear()}</div>
      <div className="mf-mini__head">{MINI_HEAD.map((h, i) => <span key={i}>{h}</span>)}</div>
      <div className="mf-mini__grid">
        {cells.map((c, i) => (
          <span key={i} onClick={() => onPick(c)} style={{ cursor: 'pointer' }}
            className={cx('mf-mini__day', sameDay(c, now) && 'mf-mini__day--today', c.getMonth() !== cursor.getMonth() && 'mf-mini__day--out')}>{c.getDate()}</span>
        ))}
      </div>
    </div>
  );
}

function CalendarSidebar({ cursor, events, hidden, onToggleCal, onPick, onOpen, onRsvp }) {
  const t = useT();
  const startToday = new Date(); startToday.setHours(0, 0, 0, 0);
  const seen = new Set();
  const upcoming = events.filter(e => new Date(e.start) >= startToday).sort((a, b) => new Date(a.start) - new Date(b.start))
    .filter(e => { if (seen.has(e.uid)) return false; seen.add(e.uid); return true; }).slice(0, 7); // one row per series
  return (
    <div className="mf-cal-side">
      <div className="mf-card mf-card--pad"><MiniMonth cursor={cursor} onPick={onPick} /></div>

      <div className="mf-card mf-card--pad">
        <div className="mf-card__title" style={{ marginBottom: 13 }}>{t('calendar.myCalendars')}</div>
        <div className="mf-callist">
          {CAL_LIST.map(([key, name]) => {
            const on = !hidden.has(key);
            const dot = CAL_COLORS[key][2];
            return (
              <div key={key} className="mf-callist__item" style={{ cursor: 'pointer', opacity: on ? 1 : 0.55 }} onClick={() => onToggleCal(key)}>
                <span className="mf-callist__check" style={{ background: on ? dot : 'transparent', border: on ? 'none' : '1.5px solid var(--hair)' }}>
                  {on && <svg width="11" height="11" viewBox="0 0 14 14" fill="none"><path d="M3 7.4l2.6 2.6L11 4.6" stroke="#fff" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" /></svg>}
                </span>
                <span className="mf-callist__name">{name}</span>
              </div>
            );
          })}
        </div>
      </div>

      <div className="mf-card mf-card--pad">
        <div className="mf-card__title" style={{ marginBottom: 13 }}>{t('calendar.upcoming')}</div>
        {upcoming.length === 0 && <div style={{ fontSize: 12.5, color: 'var(--faint)' }}>{t('calendar.noUpcoming')}</div>}
        <div className="mf-upcoming">
          {upcoming.map(ev => {
            const rsvp = ev.rsvp || 'none';
            const d = new Date(ev.start);
            return (
              <div key={ev.uid} className="mf-upcoming__item">
                <div className="mf-upcoming__date"><div className="mf-upcoming__day">{d.getDate()}</div><div className="mf-upcoming__mon">{MONTHS[d.getMonth()].slice(0, 3).toUpperCase()}</div></div>
                <span className="mf-upcoming__dot" style={{ background: calColors(ev.calendar)[2] }} />
                <div className="mf-upcoming__meta">
                  <div className="mf-upcoming__title" onClick={() => onOpen(ev)} style={{ cursor: 'pointer' }}>{ev.summary}</div>
                  <div className="mf-upcoming__time">{ev.all_day ? t('calendar.allDay') : hhmm(ev.start)}</div>
                  <div className="mf-rsvpbtns">
                    {RSVP_OPTS.map(([v, k]) => (
                      <button key={v} className={cx('mf-rsvpbtn', rsvp === v && 'mf-rsvpbtn--' + v)} onClick={e => { e.stopPropagation(); onRsvp(ev, v); }}>{t('calendar.' + k)}</button>
                    ))}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

// ---- page --------------------------------------------------------------------

const CHEV_L = <svg width="15" height="15" viewBox="0 0 20 20" fill="none"><path d="M12 5l-5 5 5 5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" /></svg>;
const CHEV_R = <svg width="15" height="15" viewBox="0 0 20 20" fill="none"><path d="M8 5l5 5-5 5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" /></svg>;

/** Full calendar page: header + Month/Week/Day panes + right sidebar. */
export function CalendarView({ onAppView }) {
  const t = useT();
  const { toast } = useToast();
  const { email } = useWebmailAuth();
  const [view, setView] = useState('month');
  const [cursor, setCursor] = useState(() => new Date());
  const [events, setEvents] = useState([]);
  const [hidden, setHidden] = useState(() => new Set());
  const [modal, setModal] = useState(null);   // { date } new | { event } edit
  const [detail, setDetail] = useState(null);
  const dragRef = useRef(null); // the event being dragged — a ref so drop reads it synchronously
  const [overKey, setOverKey] = useState(null);

  const load = useCallback(async () => {
    try { const evs = await wm.calendar.list(); setEvents(Array.isArray(evs) ? evs : []); }
    catch { setEvents([]); }
  }, []);
  useEffect(() => { load(); }, [load]);

  const syncDetail = useCallback(async () => {
    const evs = await wm.calendar.list().catch(() => null);
    if (Array.isArray(evs)) { setEvents(evs); setDetail(d => (d ? evs.find(e => e.uid === d.uid) || null : null)); }
  }, []);

  const shown = events.filter(e => !hidden.has(calKey(e.calendar)));

  function step(dir) {
    const d = new Date(cursor);
    if (view === 'month') d.setMonth(d.getMonth() + dir);
    else if (view === 'week') d.setDate(d.getDate() + 7 * dir);
    else d.setDate(d.getDate() + dir);
    setCursor(d);
  }
  const title = view === 'month' ? `${MONTHS[cursor.getMonth()]} ${cursor.getFullYear()}`
    : view === 'week' ? weekRangeLabel(cursor)
      : cursor.toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' });

  async function setEventRsvp(ev, rsvp) {
    try { await wm.calendar.setRsvp(ev.uid, rsvp); toast(t('calendar.responseUpdated')); load(); }
    catch (e) { toast(t('calendar.saveFailed'), (e && e.message) || ''); }
  }
  async function moveTo(ev, start) {
    const dur = new Date(ev.end) - new Date(ev.start);
    const end = new Date(start.getTime() + (dur > 0 ? dur : 3600000));
    try { await wm.calendar.update(ev.uid, { ...eventToReq(ev), start: start.toISOString(), end: end.toISOString() }); toast(t('calendar.moved')); load(); }
    catch (e) { toast(t('calendar.saveFailed'), (e && e.message) || ''); }
  }

  const drag = {
    overKey,
    start: (e, ev) => { dragRef.current = ev; try { e.dataTransfer.setData('text/plain', ev.uid); e.dataTransfer.effectAllowed = 'move'; } catch { /* noop */ } },
    end: () => { dragRef.current = null; setOverKey(null); },
    over: (e, key) => { if (!dragRef.current) return; e.preventDefault(); setOverKey(key); },
    dropDay: (e, day) => {
      e.preventDefault();
      const ev = dragRef.current;
      if (ev) {
        const start = ev.all_day
          ? new Date(Date.UTC(day.getFullYear(), day.getMonth(), day.getDate()))
          : (() => { const s = new Date(ev.start); return new Date(day.getFullYear(), day.getMonth(), day.getDate(), s.getHours(), s.getMinutes()); })();
        moveTo(ev, start);
      }
      dragRef.current = null; setOverKey(null);
    },
    dropHour: (e, hour) => {
      e.preventDefault();
      const ev = dragRef.current;
      if (ev) {
        const start = ev.all_day
          ? new Date(Date.UTC(cursor.getFullYear(), cursor.getMonth(), cursor.getDate()))
          : (() => { const s = new Date(ev.start); return new Date(cursor.getFullYear(), cursor.getMonth(), cursor.getDate(), hour, s.getMinutes()); })();
        moveTo(ev, start);
      }
      dragRef.current = null; setOverKey(null);
    },
  };

  const openEdit = () => { setModal({ event: detail._orig || detail }); setDetail(null); };
  const pickMini = d => setCursor(d);

  // Expand recurrences into visible occurrences for the current view's range,
  // plus a forward window for the sidebar's upcoming list.
  let rangeStart, rangeEnd;
  if (view === 'month') { const w = monthGrid(cursor.getFullYear(), cursor.getMonth()); rangeStart = w[0][0]; rangeEnd = addDays(w[5][6], 1); }
  else if (view === 'week') { const wd = weekDays(cursor); rangeStart = wd[0]; rangeEnd = addDays(wd[6], 1); }
  else { rangeStart = dayOf(cursor); rangeEnd = addDays(cursor, 1); }
  const instances = expandInstances(shown, rangeStart, rangeEnd);
  const todayStart = dayOf(new Date());
  const upcomingInstances = expandInstances(shown, todayStart, addDays(todayStart, 90));

  return (
    <div>
      <div className="mf-cal-head">
        <div>
          <h1 className="mf-page-head__title" style={{ fontSize: 27 }}>{t('calendar.calendarTitle')}</h1>
          <div className="mf-page-head__sub">{title} · {email}</div>
        </div>
        <Segmented className="mf-cal-appseg" options={[t('webmail.view.mail'), t('webmail.view.calendar')]} value={t('webmail.view.calendar')}
          onSelect={v => onAppView && onAppView(v === t('webmail.view.mail') ? 'mail' : 'calendar')} />
        <div className="mf-cal-headright">
          <div className="mf-daynav">
            <button className="mf-daynav__arrow" aria-label="Previous" onClick={() => step(-1)}>{CHEV_L}</button>
            <button className="mf-daynav__today" onClick={() => setCursor(new Date())}>{t('calendar.today')}</button>
            <button className="mf-daynav__arrow" aria-label="Next" onClick={() => step(1)}>{CHEV_R}</button>
          </div>
          <Segmented options={[{ label: t('calendar.viewMonth'), value: 'month' }, { label: t('calendar.viewWeek'), value: 'week' }, { label: t('calendar.viewDay'), value: 'day' }]} value={view} onSelect={setView} />
          <Button variant="primary" size="sm" onClick={() => setModal({ date: cursor })}>
            <svg width="14" height="14" viewBox="0 0 20 20" fill="none" style={{ marginRight: 6, verticalAlign: '-2px' }}><path d="M10 4v12M4 10h12" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" /></svg>{t('calendar.newEvent')}
          </Button>
        </div>
      </div>

      <div className="mf-cal-layout">
        {view === 'month' && <MonthView cursor={cursor} events={instances} onOpen={setDetail} onNew={d => setModal({ date: d })} drag={drag} />}
        {view === 'week' && <WeekView cursor={cursor} events={instances} onOpen={setDetail} drag={drag} />}
        {view === 'day' && <DayView cursor={cursor} events={instances} onOpen={setDetail} drag={drag} />}
        <CalendarSidebar cursor={cursor} events={upcomingInstances} hidden={hidden} onPick={pickMini} onOpen={setDetail} onRsvp={setEventRsvp}
          onToggleCal={key => setHidden(h => { const n = new Set(h); n.has(key) ? n.delete(key) : n.add(key); return n; })} />
      </div>

      {modal && <EventModal date={modal.date} event={modal.event} onClose={() => setModal(null)} onSaved={load} />}
      {detail && <EventDetail ev={detail} onClose={() => setDetail(null)} onChanged={syncDetail} onDeleted={load} onEdit={openEdit} />}
    </div>
  );
}
