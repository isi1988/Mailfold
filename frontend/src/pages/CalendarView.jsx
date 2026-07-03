import React, { useState, useEffect, useCallback } from 'react';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { IconButton } from '../ds/components/atoms/IconButton.jsx';
import { Toggle } from '../ds/components/atoms/Toggle.jsx';
import { wm } from '../api/webmail.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';

const WEEKDAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
const MAX_ATTACH_TOTAL = 10 * 1024 * 1024; // keep in step with the backend cap

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

// readFileAsBase64 resolves to the file's base64 payload (without the data: prefix).
const readFileAsBase64 = file => new Promise((resolve, reject) => {
  const r = new FileReader();
  r.onerror = () => reject(new Error('read failed'));
  r.onload = () => resolve(String(r.result).split(',')[1] || '');
  r.readAsDataURL(file);
});

// monthGrid returns six Monday-started weeks of Date objects covering the month.
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

// GuestsInput is a lightweight chip field for the event's invitees.
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
          {g}
          <span onClick={() => onChange(value.filter(x => x !== g))} style={{ cursor: 'pointer', opacity: 0.7, fontWeight: 700 }}>×</span>
        </span>
      ))}
      <input value={text} onChange={e => setText(e.target.value)} onKeyDown={key} onBlur={add}
        placeholder={value.length ? '' : placeholder}
        style={{ flex: 1, minWidth: 120, border: 'none', outline: 'none', background: 'transparent', color: 'var(--ink)', font: '400 13.5px system-ui' }} />
    </div>
  );
}

// EventModal is the full "New event" form, laid out 1:1 with the design and
// extended with file attachments.
function EventModal({ date, onClose, onSaved }) {
  const t = useT();
  const { toast } = useToast();
  const [summary, setSummary] = useState('');
  const [allDay, setAllDay] = useState(false);
  const [startDate, setStartDate] = useState(ymd(date));
  const [startTime, setStartTime] = useState('09:30');
  const [endDate, setEndDate] = useState(ymd(date));
  const [endTime, setEndTime] = useState('10:30');
  const [calendar, setCalendar] = useState('Work');
  const [location, setLocation] = useState('');
  const [guests, setGuests] = useState([]);
  const [description, setDescription] = useState('');
  const [attachments, setAttachments] = useState([]); // { filename, mime, size, data }
  const [repeat, setRepeat] = useState('');
  const [reminder, setReminder] = useState(10);
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
        start = new Date(startDate + 'T00:00');
        end = new Date(endDate + 'T00:00');
        if (!(end > start)) end = new Date(start.getTime() + 24 * 60 * 60 * 1000);
      } else {
        start = new Date(startDate + 'T' + startTime);
        end = new Date(endDate + 'T' + endTime);
        if (!(end > start)) end = new Date(start.getTime() + 60 * 60 * 1000);
      }
      await wm.calendar.create({
        summary: summary.trim(), start: start.toISOString(), end: end.toISOString(),
        all_day: allDay, calendar, location: location.trim(), guests,
        description, repeat, reminder,
        attachments: attachments.map(a => ({ filename: a.filename, mime: a.mime, data: a.data })),
      });
      toast(t('calendar.created'));
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
        {/* header */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '16px 24px', borderBottom: '1px solid var(--hair-soft)', background: 'var(--surface-2)', flex: 'none' }}>
          <div style={{ width: 32, height: 32, borderRadius: 9, background: 'var(--accent-soft)', display: 'flex', alignItems: 'center', justifyContent: 'center', flex: 'none' }}>
            <svg width="17" height="17" viewBox="0 0 20 20" fill="none"><rect x="3" y="4.5" width="14" height="12" rx="2" stroke="var(--accent-ink)" strokeWidth="1.4" /><path d="M3 8.2h14M7 3v3M13 3v3" stroke="var(--accent-ink)" strokeWidth="1.4" strokeLinecap="round" /></svg>
          </div>
          <div style={{ fontFamily: 'var(--font-serif)', fontSize: 21, fontWeight: 600, color: 'var(--ink-strong)' }}>{t('calendar.newEvent')}</div>
          <div onClick={onClose} style={{ marginLeft: 'auto', cursor: 'pointer', color: 'var(--faint)', display: 'flex', alignItems: 'center', gap: 6, padding: '7px 12px', borderRadius: 9, font: '500 12.5px system-ui' }}>
            <svg width="16" height="16" viewBox="0 0 20 20" fill="none"><path d="M5 5l10 10M15 5L5 15" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" /></svg>
            {t('common.close')}
          </div>
        </div>

        {/* body */}
        <div style={{ padding: '22px 24px', display: 'flex', flexDirection: 'column', gap: 16, overflowY: 'auto' }}>
          <input placeholder={t('calendar.titlePlaceholder')} value={summary} onChange={e => setSummary(e.target.value)} autoFocus
            style={{ width: '100%', border: 'none', borderBottom: '2px solid var(--hair)', background: 'transparent', outline: 'none', font: '600 20px var(--font-serif)', color: 'var(--ink-strong)', padding: '6px 2px', boxSizing: 'border-box' }} />

          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '2px 0' }}>
            <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>{t('calendar.allDay')}</div>
            <Toggle on={allDay} onClick={() => setAllDay(v => !v)} style={{ cursor: 'pointer' }} />
          </div>

          <div style={{ display: 'flex', gap: 12 }}>
            <div style={{ flex: 1 }}>
              <label style={FIELD_LABEL}>{t('calendar.starts')}</label>
              <input type="date" value={startDate} onChange={e => setStartDate(e.target.value)} style={FIELD} />
            </div>
            <div style={{ width: 120 }}>
              <label style={FIELD_LABEL}>{t('calendar.time')}</label>
              <input type="time" value={startTime} onChange={e => setStartTime(e.target.value)} disabled={allDay} style={{ ...FIELD, fontFamily: 'var(--font-mono)', opacity: allDay ? 0.5 : 1 }} />
            </div>
          </div>

          <div style={{ display: 'flex', gap: 12 }}>
            <div style={{ flex: 1 }}>
              <label style={FIELD_LABEL}>{t('calendar.ends')}</label>
              <input type="date" value={endDate} onChange={e => setEndDate(e.target.value)} style={FIELD} />
            </div>
            <div style={{ width: 120 }}>
              <label style={FIELD_LABEL}>{t('calendar.time')}</label>
              <input type="time" value={endTime} onChange={e => setEndTime(e.target.value)} disabled={allDay} style={{ ...FIELD, fontFamily: 'var(--font-mono)', opacity: allDay ? 0.5 : 1 }} />
            </div>
          </div>

          <div>
            <label style={FIELD_LABEL}>{t('calendar.calendar')}</label>
            <select className="mf-input" value={calendar} onChange={e => setCalendar(e.target.value)}>
              <option value="Work">Work</option>
              <option value="Personal">Personal</option>
              <option value="Team">Team</option>
              <option value="Holidays">Holidays</option>
            </select>
          </div>

          <div>
            <label style={FIELD_LABEL}>{t('calendar.location')}</label>
            <input placeholder={t('calendar.locationPlaceholder')} value={location} onChange={e => setLocation(e.target.value)} style={FIELD} />
          </div>

          <div>
            <label style={FIELD_LABEL}>{t('calendar.guests')}</label>
            <GuestsInput value={guests} onChange={setGuests} placeholder={t('calendar.guestsPlaceholder')} />
          </div>

          <div>
            <label style={FIELD_LABEL}>{t('calendar.description')}</label>
            <textarea placeholder={t('calendar.descriptionPlaceholder')} value={description} onChange={e => setDescription(e.target.value)}
              style={{ ...FIELD, minHeight: 90, lineHeight: 1.6, resize: 'vertical' }} />
          </div>

          {/* attachments */}
          <div>
            <label style={FIELD_LABEL}>{t('calendar.attachments')}</label>
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
              {t('calendar.attachFiles')}
              <input type="file" multiple onChange={pickFiles} style={{ display: 'none' }} />
            </label>
          </div>

          <div style={{ display: 'flex', gap: 12 }}>
            <div style={{ flex: 1 }}>
              <label style={FIELD_LABEL}>{t('calendar.repeat')}</label>
              <select className="mf-input" value={repeat} onChange={e => setRepeat(e.target.value)}>
                {REPEATS.map(([v, k]) => <option key={k} value={v}>{t('calendar.' + k)}</option>)}
              </select>
            </div>
            <div style={{ flex: 1 }}>
              <label style={FIELD_LABEL}>{t('calendar.reminder')}</label>
              <select className="mf-input" value={reminder} onChange={e => setReminder(Number(e.target.value))}>
                {REMINDERS.map(([v, k]) => <option key={k} value={v}>{t('calendar.' + k)}</option>)}
              </select>
            </div>
          </div>
        </div>

        {/* footer */}
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10, padding: '14px 24px', borderTop: '1px solid var(--hair-soft)', background: 'var(--surface-2)', flex: 'none' }}>
          <Button variant="secondary" onClick={onClose}>{t('common.cancel')}</Button>
          <Button variant="primary" onClick={save} disabled={busy}>{busy ? t('common.saving') : t('calendar.save')}</Button>
        </div>
      </div>
    </div>
  );
}

// ---- category colours + RSVP -------------------------------------------------

// CAL_COLORS keys a calendar category to [soft background, ink, dot] tokens.
const CAL_COLORS = {
  work: ['var(--accent-soft)', 'var(--accent-ink)', 'var(--accent)'],
  personal: ['var(--green-soft)', 'var(--green)', 'var(--green)'],
  team: ['var(--blue-soft)', 'var(--blue)', 'var(--blue)'],
  holiday: ['var(--amber-soft)', 'var(--amber)', 'var(--amber)'],
  holidays: ['var(--amber-soft)', 'var(--amber)', 'var(--amber)'],
};
const calColors = cal => CAL_COLORS[(cal || 'work').toLowerCase()] || CAL_COLORS.work;
const calName = cal => ({ work: 'Work', personal: 'Personal', team: 'Team', holiday: 'Holidays', holidays: 'Holidays' }[(cal || 'work').toLowerCase()] || cal || 'Work');
const RSVP_OPTS = [['yes', 'rsvpGoing'], ['maybe', 'rsvpMaybe'], ['no', 'rsvpCant']];

// RsvpMark is the Outlook-style response glyph: ✓ going / ? maybe / ✕ not going.
function RsvpMark({ rsvp, size = 13 }) {
  const box = { display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: size, height: size, flex: 'none', borderRadius: '50%', lineHeight: 1 };
  if (rsvp === 'yes') return <span title="Going" style={{ ...box, color: 'var(--green)' }}><svg width="8" height="8" viewBox="0 0 12 12" fill="none"><path d="M2.4 6.4l2.2 2.2 5-5.2" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" /></svg></span>;
  if (rsvp === 'no') return <span title="Not going" style={{ ...box, color: 'var(--red)' }}><svg width="8" height="8" viewBox="0 0 12 12" fill="none"><path d="M3 3l6 6M9 3l-6 6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" /></svg></span>;
  if (rsvp === 'maybe') return <span title="Maybe" style={{ ...box, width: size - 1, height: size - 1, border: '1.2px solid currentColor', opacity: 0.7, fontFamily: 'var(--font-mono)', fontWeight: 700, fontSize: 8 }}>?</span>;
  return null;
}

// EventChip is a coloured month-grid event pill: category colour, RSVP mark, and
// hatched (maybe) / struck-through (declined) styling.
function EventChip({ ev, onOpen }) {
  const [bg, ink, dot] = calColors(ev.calendar);
  const rsvp = ev.rsvp || 'none';
  const style = { marginTop: 3, display: 'flex', alignItems: 'center', gap: 5, padding: '2px 7px', borderRadius: 6, fontSize: 11, fontWeight: 500, cursor: 'pointer', overflow: 'hidden', background: bg, color: ink };
  if (rsvp === 'maybe') style.backgroundImage = 'repeating-linear-gradient(45deg, transparent 0 3px, color-mix(in srgb, currentColor 14%, transparent) 3px 6px)';
  if (rsvp === 'no') style.opacity = 0.6;
  return (
    <div title={ev.summary} onClick={onOpen} style={style}>
      {rsvp === 'none'
        ? <span style={{ width: 5, height: 5, borderRadius: '50%', flex: 'none', background: dot }} />
        : <RsvpMark rsvp={rsvp} size={12} />}
      <span style={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', textDecoration: rsvp === 'no' ? 'line-through' : 'none' }}>
        {!ev.all_day && <span className="mf-u-mono">{hhmm(ev.start)}&nbsp;&nbsp;</span>}{ev.summary}
      </span>
    </div>
  );
}

// RsvpButton is one of the Going / Maybe / Can't response buttons.
function RsvpButton({ kind, active, label, onClick }) {
  const on = { yes: ['var(--green-soft)', 'var(--green)'], maybe: ['var(--accent-soft)', 'var(--accent-ink)'], no: ['var(--red-soft)', 'var(--red)'] }[kind];
  const skin = active ? { background: on[0], color: on[1], border: '1px solid transparent' } : { background: 'var(--surface)', color: 'var(--muted)', border: '1px solid var(--hair)' };
  return <button onClick={onClick} style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', padding: '6px 12px', borderRadius: 8, font: '600 12px system-ui', cursor: 'pointer', ...skin }}>{label}</button>;
}

// MetaRow is a label/value line in the event detail view.
function MetaRow({ label, value }) {
  return (
    <div style={{ display: 'flex', gap: 12, fontSize: 13 }}>
      <span style={{ flex: 'none', width: 74, color: 'var(--faint)', fontWeight: 600 }}>{label}</span>
      <span style={{ flex: 1, minWidth: 0, color: 'var(--ink)', whiteSpace: 'pre-wrap' }}>{value}</span>
    </div>
  );
}

// EventDetail shows a single event with its metadata and the owner's RSVP row.
function EventDetail({ ev, onClose, onChanged, onDeleted }) {
  const t = useT();
  const { toast } = useToast();
  const [rsvp, setRsvp] = useState(ev.rsvp || 'none');
  const [busy, setBusy] = useState(false);
  const [bg, ink, dot] = calColors(ev.calendar);

  async function choose(v) {
    const prev = rsvp;
    setRsvp(v);
    try {
      await wm.calendar.setRsvp(ev.uid, v);
      toast(t('calendar.responseUpdated'));
      if (onChanged) onChanged();
    } catch (e) {
      setRsvp(prev);
      toast(t('calendar.saveFailed'), (e && e.message) || '');
    }
  }

  async function del() {
    if (busy) return;
    if (!window.confirm(t('calendar.deleteConfirm', { title: ev.summary }))) return;
    setBusy(true);
    try {
      await wm.calendar.del(ev.uid);
      toast(t('calendar.deleted'));
      if (onDeleted) onDeleted();
      onClose();
    } catch (e) {
      toast(t('calendar.saveFailed'), (e && e.message) || '');
      setBusy(false);
    }
  }

  const dateLabel = new Date(ev.start).toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' });
  const timeLabel = ev.all_day ? t('calendar.allDay') : hhmm(ev.start) + (ev.end ? ' – ' + hhmm(ev.end) : '');
  const repeatKey = ev.repeat && 'repeat' + ev.repeat.charAt(0) + ev.repeat.slice(1).toLowerCase();

  return (
    <div className="mf-overlay mf-overlay--center" onClick={onClose}>
      <div onClick={e => e.stopPropagation()} style={{ width: 'min(520px, 94vw)', maxHeight: '92vh', display: 'flex', flexDirection: 'column', background: 'var(--surface)', border: '1px solid var(--hair)', borderRadius: 16, boxShadow: '0 34px 90px rgba(0,0,0,.34)', overflow: 'hidden' }}>
        <div style={{ height: 6, background: dot, flex: 'none' }} />
        <div style={{ padding: '20px 24px', display: 'flex', flexDirection: 'column', gap: 14, overflowY: 'auto' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: 7, font: '600 12px system-ui', padding: '5px 12px', borderRadius: 999, background: bg, color: ink }}>
              <span style={{ width: 9, height: 9, borderRadius: '50%', background: dot }} />{calName(ev.calendar)}
            </span>
            <div onClick={onClose} style={{ marginLeft: 'auto', cursor: 'pointer', color: 'var(--faint)', display: 'flex' }}>
              <svg width="18" height="18" viewBox="0 0 20 20" fill="none"><path d="M5 5l10 10M15 5L5 15" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" /></svg>
            </div>
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
                <div key={i} onClick={() => wm.calendar.downloadAttachment(ev.uid, i, a.filename)}
                  style={{ display: 'flex', alignItems: 'center', gap: 9, padding: '8px 11px', border: '1px solid var(--hair)', borderRadius: 9, background: 'var(--surface-2)', cursor: 'pointer' }}>
                  <svg width="15" height="15" viewBox="0 0 20 20" fill="none" style={{ flex: 'none', color: 'var(--faint)' }}><path d="M8 3.5v9a3 3 0 006 0V5a2 2 0 10-4 0v7.5a1 1 0 002 0V6" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" /></svg>
                  <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 13, color: 'var(--ink)' }}>{a.filename}</span>
                  <span className="mf-u-mono" style={{ fontSize: 11.5, color: 'var(--faint)' }}>{humanSize(a.size || 0)}</span>
                </div>
              ))}
            </div>
          )}

          <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 4, paddingTop: 16, borderTop: '1px solid var(--hair-soft)' }}>
            <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>{t('calendar.yourResponse')}</span>
            <div style={{ display: 'flex', gap: 8, marginLeft: 'auto' }}>
              {RSVP_OPTS.map(([v, k]) => <RsvpButton key={v} kind={v} active={rsvp === v} label={t('calendar.' + k)} onClick={() => choose(v)} />)}
            </div>
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '14px 24px', borderTop: '1px solid var(--hair-soft)', background: 'var(--surface-2)', flex: 'none' }}>
          <Button variant="danger" onClick={del} disabled={busy}>{t('common.delete')}</Button>
          <div style={{ flex: 1 }} />
          <Button variant="secondary" onClick={onClose}>{t('common.close')}</Button>
        </div>
      </div>
    </div>
  );
}

/** Month-view calendar over the mailbox's CalDAV events. */
export function CalendarView() {
  const t = useT();
  const { toast } = useToast();
  const [cursor, setCursor] = useState(() => new Date());
  const [events, setEvents] = useState([]);
  const [modal, setModal] = useState(null); // { date } for a new event
  const [detail, setDetail] = useState(null); // the event being viewed

  const load = useCallback(async () => {
    try {
      const evs = await wm.calendar.list();
      setEvents(Array.isArray(evs) ? evs : []);
    } catch {
      setEvents([]);
    }
  }, []);
  useEffect(() => { load(); }, [load]);

  const year = cursor.getFullYear();
  const month = cursor.getMonth();
  const weeks = monthGrid(year, month);
  const now = new Date();

  // Keep the open detail card in sync after an RSVP change.
  const syncDetail = useCallback(async () => {
    const evs = await wm.calendar.list().catch(() => null);
    if (Array.isArray(evs)) {
      setEvents(evs);
      setDetail(d => (d ? evs.find(e => e.uid === d.uid) || null : null));
    }
  }, []);

  return (
    <div className="mf-webmail" style={{ height: 'calc(100vh - 190px)', minHeight: 460, border: '1px solid var(--hair)', borderRadius: 12, overflow: 'hidden', background: 'var(--surface)', display: 'flex', flexDirection: 'column' }}>
      <div className="mf-webmail__toolbar" style={{ gap: 10 }}>
        <div style={{ fontFamily: 'var(--font-serif)', fontSize: 20, fontWeight: 600, color: 'var(--ink-strong)' }}>{MONTHS[month]} {year}</div>
        <Button variant="secondary" size="sm" onClick={() => setCursor(new Date())}>{t('calendar.today')}</Button>
        <IconButton onClick={() => setCursor(new Date(year, month - 1, 1))}><Icon name="chevron-left" size={16} /></IconButton>
        <IconButton onClick={() => setCursor(new Date(year, month + 1, 1))}><Icon name="chevron-right" size={16} /></IconButton>
        <div className="mf-spacer" />
        <Button variant="primary" size="sm" onClick={() => setModal({ date: new Date() })}>
          <svg width="14" height="14" viewBox="0 0 20 20" fill="none" style={{ marginRight: 6, verticalAlign: '-2px' }}><path d="M10 4v12M4 10h12" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" /></svg>
          {t('calendar.newEvent')}
        </Button>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7,1fr)', borderBottom: '1px solid var(--hair)' }}>
        {WEEKDAYS.map(d => (
          <div key={d} style={{ padding: '11px 8px', textAlign: 'center', fontSize: 11, fontWeight: 600, color: 'var(--faint)', textTransform: 'uppercase', letterSpacing: '.06em' }}>{d}</div>
        ))}
      </div>

      <div style={{ flex: 1, display: 'grid', gridTemplateRows: 'repeat(6,1fr)', overflow: 'auto' }}>
        {weeks.map((week, wi) => (
          <div key={wi} style={{ display: 'grid', gridTemplateColumns: 'repeat(7,1fr)' }}>
            {week.map((day, di) => {
              const inMonth = day.getMonth() === month;
              const weekend = di >= 5;
              const isToday = sameDay(day, now);
              const dayEvents = events.filter(e => sameDay(new Date(e.start), day)).sort((a, b) => new Date(a.start) - new Date(b.start));
              const cellBg = !inMonth ? 'var(--bg)' : (weekend ? 'var(--surface-2)' : 'transparent');
              return (
                <div key={di} onClick={() => setModal({ date: day })}
                  style={{ borderRight: '1px solid var(--hair-soft)', borderBottom: '1px solid var(--hair-soft)', padding: '7px 8px', minHeight: 84, cursor: 'pointer', background: cellBg, overflow: 'hidden' }}>
                  <div style={{ alignSelf: 'flex-start', minWidth: 22, height: 22, padding: '0 5px', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', borderRadius: 11, marginBottom: 1, fontSize: 12, fontWeight: 600, color: isToday ? '#fff' : (inMonth ? 'var(--ink)' : 'var(--faint)'), background: isToday ? 'var(--accent)' : 'transparent' }}>{day.getDate()}</div>
                  {dayEvents.slice(0, 3).map(ev => (
                    <EventChip key={ev.uid} ev={ev} onOpen={e => { e.stopPropagation(); setDetail(ev); }} />
                  ))}
                  {dayEvents.length > 3 && <div style={{ fontSize: 10.5, color: 'var(--faint)', marginTop: 2, paddingLeft: 3 }}>{t('calendar.moreCount', { n: dayEvents.length - 3 })}</div>}
                </div>
              );
            })}
          </div>
        ))}
      </div>

      {modal && <EventModal date={modal.date} onClose={() => setModal(null)} onSaved={load} />}
      {detail && <EventDetail ev={detail} onClose={() => setDetail(null)} onChanged={syncDetail} onDeleted={load} />}
    </div>
  );
}
