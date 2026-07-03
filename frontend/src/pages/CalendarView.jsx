import React, { useState, useEffect, useCallback } from 'react';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { IconButton } from '../ds/components/atoms/IconButton.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Toggle } from '../ds/components/atoms/Toggle.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { wm } from '../api/webmail.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';

const WEEKDAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];

const pad = n => String(n).padStart(2, '0');
const ymd = d => d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate());
const sameDay = (a, b) => a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
const hhmm = iso => { const d = new Date(iso); return isNaN(d) ? '' : pad(d.getHours()) + ':' + pad(d.getMinutes()); };

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

// EventModal: create a calendar event, centred to match the design.
function EventModal({ date, onClose, onSaved }) {
  const t = useT();
  const { toast } = useToast();
  const [summary, setSummary] = useState('');
  const [dateStr, setDateStr] = useState(ymd(date));
  const [startTime, setStartTime] = useState('09:00');
  const [endTime, setEndTime] = useState('10:00');
  const [allDay, setAllDay] = useState(false);
  const [location, setLocation] = useState('');
  const [busy, setBusy] = useState(false);

  async function save() {
    if (busy) return;
    if (!summary.trim()) { toast(t('calendar.needTitle')); return; }
    setBusy(true);
    try {
      const start = new Date(dateStr + 'T' + (allDay ? '00:00' : startTime));
      const end = new Date(dateStr + 'T' + (allDay ? '23:59' : endTime));
      await wm.calendar.create({ summary: summary.trim(), start: start.toISOString(), end: end.toISOString(), all_day: allDay, location: location.trim() });
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
      <div onClick={e => e.stopPropagation()} style={{ width: 'min(460px, 94vw)', background: 'var(--surface)', border: '1px solid var(--hair)', borderRadius: 16, boxShadow: 'var(--shadow-modal)', overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
        <div className="mf-drawer__head">
          <div className="mf-drawer__title">{t('calendar.newEvent')}</div>
          <div className="mf-modal-close mf-spacer" onClick={onClose}><Icon name="close" size={18} /></div>
        </div>
        <div style={{ padding: '16px 22px', display: 'flex', flexDirection: 'column', gap: 14 }}>
          <Input size="lg" placeholder={t('calendar.titlePlaceholder')} value={summary} onChange={e => setSummary(e.target.value)} />
          <FormField label={t('calendar.date')}>
            <input type="date" className="mf-input" value={dateStr} onChange={e => setDateStr(e.target.value)} />
          </FormField>
          <div className="mf-row mf-row--between">
            <span className="mf-u-muted" style={{ fontSize: 13 }}>{t('calendar.allDay')}</span>
            <Toggle on={allDay} onClick={() => setAllDay(a => !a)} style={{ cursor: 'pointer' }} />
          </div>
          {!allDay && (
            <div className="mf-row" style={{ gap: 10 }}>
              <FormField label={t('calendar.starts')} style={{ flex: 1 }}>
                <input type="time" className="mf-input" value={startTime} onChange={e => setStartTime(e.target.value)} />
              </FormField>
              <FormField label={t('calendar.ends')} style={{ flex: 1 }}>
                <input type="time" className="mf-input" value={endTime} onChange={e => setEndTime(e.target.value)} />
              </FormField>
            </div>
          )}
          <FormField label={t('calendar.location')}>
            <Input placeholder={t('calendar.locationPlaceholder')} value={location} onChange={e => setLocation(e.target.value)} />
          </FormField>
        </div>
        <div className="mf-drawer__foot">
          <Button variant="primary" onClick={save} disabled={busy}>{busy ? t('common.saving') : t('calendar.save')}</Button>
          <Button variant="link" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
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
    <div className="mf-webmail" style={{ height: 'calc(100vh - 150px)', minHeight: 460, border: '1px solid var(--hair)', borderRadius: 12, overflow: 'hidden', background: 'var(--surface)', display: 'flex', flexDirection: 'column' }}>
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
