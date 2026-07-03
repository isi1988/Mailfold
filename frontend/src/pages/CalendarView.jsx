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

/** Month-view calendar over the mailbox's CalDAV events. */
export function CalendarView() {
  const t = useT();
  const { toast } = useToast();
  const [cursor, setCursor] = useState(() => new Date());
  const [events, setEvents] = useState([]);
  const [modal, setModal] = useState(null); // { date } for a new event

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

  async function removeEvent(ev) {
    if (!window.confirm(t('calendar.deleteConfirm', { title: ev.summary }))) return;
    try {
      await wm.calendar.del(ev.uid);
      toast(t('calendar.deleted'));
      load();
    } catch (e) {
      toast(t('calendar.saveFailed'), (e && e.message) || '');
    }
  }

  return (
    <div className="mf-webmail" style={{ height: 'calc(100vh - 190px)', minHeight: 460, border: '1px solid var(--hair)', borderRadius: 12, overflow: 'hidden', background: 'var(--surface)', display: 'flex', flexDirection: 'column' }}>
      <div className="mf-webmail__toolbar" style={{ gap: 10 }}>
        <div style={{ fontFamily: 'var(--font-serif)', fontSize: 20, fontWeight: 600, color: 'var(--ink-strong)' }}>{MONTHS[month]} {year}</div>
        <Button variant="secondary" size="sm" onClick={() => setCursor(new Date())}>{t('calendar.today')}</Button>
        <IconButton onClick={() => setCursor(new Date(year, month - 1, 1))}><Icon name="chevron-left" size={16} /></IconButton>
        <IconButton onClick={() => setCursor(new Date(year, month + 1, 1))}><Icon name="chevron-right" size={16} /></IconButton>
        <div className="mf-spacer" />
        <Button variant="primary" size="sm" onClick={() => setModal({ date: new Date() })}>{t('calendar.newEvent')}</Button>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7,1fr)', borderBottom: '1px solid var(--hair)' }}>
        {WEEKDAYS.map(d => (
          <div key={d} style={{ padding: '8px 10px', fontSize: 11, fontWeight: 600, color: 'var(--faint)', textTransform: 'uppercase', letterSpacing: '.05em' }}>{d}</div>
        ))}
      </div>

      <div style={{ flex: 1, display: 'grid', gridTemplateRows: 'repeat(6,1fr)', overflow: 'auto' }}>
        {weeks.map((week, wi) => (
          <div key={wi} style={{ display: 'grid', gridTemplateColumns: 'repeat(7,1fr)' }}>
            {week.map((day, di) => {
              const inMonth = day.getMonth() === month;
              const isToday = sameDay(day, now);
              const dayEvents = events.filter(e => sameDay(new Date(e.start), day)).sort((a, b) => new Date(a.start) - new Date(b.start));
              return (
                <div key={di} onClick={() => setModal({ date: day })}
                  style={{ borderRight: '1px solid var(--hair-soft)', borderBottom: '1px solid var(--hair-soft)', padding: 6, minHeight: 78, cursor: 'pointer', background: inMonth ? 'transparent' : 'var(--surface-2)' }}>
                  <div style={{ fontSize: 12, fontWeight: 600, width: 22, height: 22, display: 'flex', alignItems: 'center', justifyContent: 'center', borderRadius: '50%', color: isToday ? '#fff' : (inMonth ? 'var(--ink)' : 'var(--faint)'), background: isToday ? 'var(--accent)' : 'transparent' }}>{day.getDate()}</div>
                  {dayEvents.slice(0, 3).map(ev => (
                    <div key={ev.uid} title={ev.summary} onClick={e => { e.stopPropagation(); removeEvent(ev); }}
                      style={{ marginTop: 3, fontSize: 11, padding: '2px 6px', borderRadius: 5, background: 'var(--accent-soft)', color: 'var(--accent-ink)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {!ev.all_day && <span className="mf-u-mono">{hhmm(ev.start)} </span>}{ev.summary}
                    </div>
                  ))}
                  {dayEvents.length > 3 && <div style={{ fontSize: 10, color: 'var(--faint)', marginTop: 2 }}>+{dayEvents.length - 3}</div>}
                </div>
              );
            })}
          </div>
        ))}
      </div>

      {modal && <EventModal date={modal.date} onClose={() => setModal(null)} onSaved={load} />}
    </div>
  );
}
