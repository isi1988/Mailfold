import React, { useState, useEffect, useRef, useCallback } from 'react';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { SearchInput } from '../ds/components/molecules/SearchInput.jsx';
import { Tabs } from '../ds/components/molecules/Tabs.jsx';
import { Card } from '../ds/components/molecules/Card.jsx';
import { Pill } from '../ds/components/atoms/Pill.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { tone } from '../ds/data/sample.js';
import { api } from '../api/client.js';
import { AsyncView } from '../components/States.jsx';
import { asList } from '../lib/format.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';

// The mailcow log sources exposed by the /api/logs/{service} endpoint (mirrors
// the backend's validLogServices allowlist in internal/api/logs.go).
const SERVICES = ['postfix', 'dovecot', 'rspamd-history', 'sogo', 'netfilter', 'watchdog', 'ratelimited', 'autodiscover', 'acme', 'api'];
const ALL = '__all';
const REFRESH_MS = 5000;

// Collapse an arbitrary syslog priority string into a Pill tone bucket. A
// dedicated "reject" bucket is kept apart from generic errors so a spam/policy
// rejection reads differently from a service crash at a glance.
function severity(priority) {
  const p = String(priority || '').toLowerCase();
  if (/reject/.test(p)) return 'reject';
  if (/(err|crit|alert|emerg|fatal|panic)/.test(p)) return 'error';
  if (/(warn|notice)/.test(p)) return 'warn';
  return 'info';
}

// mailcow returns time as unix seconds encoded as a string.
function formatTime(time) {
  const secs = Number(time);
  if (!secs) return '—';
  const d = new Date(secs * 1000);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleString(undefined, {
    month: 'short', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit',
  });
}

// rspamd-history entries have a completely different shape (scan verdicts, not
// syslog lines) — normalize them into the same {time, program, priority,
// message} shape every other service already uses.
function normalize(service, row) {
  if (service !== 'rspamd-history') return { ...row, service };
  const rcpts = asList(row.rcpt_smtp).join(', ') || '—';
  const action = row.action || 'no action';
  return {
    service,
    time: row.unix_time,
    program: 'rspamd',
    priority: action,
    message: `${row.sender_smtp || '—'} → ${rcpts} · ${action} (score ${row.score ?? '—'}/${row.required_score ?? '—'})`,
  };
}

export function LogsPage() {
  const t = useT();
  const { toast } = useToast();
  const [service, setService] = useState(SERVICES[0]);
  const [q, setQ] = useState('');
  const [live, setLive] = useState(false);
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const timerRef = useRef(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      if (service === ALL) {
        const results = await Promise.all(SERVICES.map(s => api.get('/api/logs/' + s).catch(() => [])));
        const merged = SERVICES.flatMap((s, i) => asList(results[i]).map(r => normalize(s, r)));
        merged.sort((a, b) => Number(b.time) - Number(a.time));
        setRows(merged);
      } else {
        const data = await api.get('/api/logs/' + service);
        setRows(asList(data).map(r => normalize(service, r)));
      }
    } catch (e) {
      setError(e);
    } finally {
      setLoading(false);
    }
  }, [service]);

  useEffect(() => { load(); }, [load]);

  // Auto-refresh while "Live" is on, only while the tab is visible.
  useEffect(() => {
    if (!live) return undefined;
    timerRef.current = setInterval(() => {
      if (document.visibilityState === 'visible') load();
    }, REFRESH_MS);
    return () => clearInterval(timerRef.current);
  }, [live, load]);

  const filtered = q
    ? rows.filter(l => {
        const hay = (
          (l.program || '') + ' ' + (l.priority || '') + ' ' + (l.message || '')
        ).toLowerCase();
        return hay.includes(q.toLowerCase());
      })
    : rows;

  // Switching the service tab refetches and clears the filter.
  const onService = id => { setQ(''); setService(id); };

  // Purely client-side download of the currently fetched (unfiltered) lines.
  function download() {
    if (rows.length === 0) return;
    const text = rows
      .map(l => `${formatTime(l.time)}\t${l.priority || ''}\t${l.program || ''}\t${l.message || ''}`)
      .join('\n');
    const url = URL.createObjectURL(new Blob([text], { type: 'text/plain' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = `mailfold-${service}-logs.txt`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
    toast(t('logs.downloaded', { service }));
  }

  return (
    <>
      <PageHeader
        title={t('logs.title')}
        sub={t('logs.sub', { service: service === ALL ? t('logs.allServices') : service })}
        actions={
          <>
            <Button variant={live ? 'primary' : 'secondary'} onClick={() => setLive(v => !v)}>
              {live && <span className="mf-dot mf-dot--pulse" style={{ marginRight: 7, background: '#fff' }} />}
              {live ? t('logs.liveOn') : t('logs.liveOff')}
            </Button>
            <Button variant="secondary" onClick={load}>{t('logs.refresh')}</Button>
            <Button variant="secondary" onClick={download} disabled={rows.length === 0}>{t('logs.download')}</Button>
          </>
        }
      />

      <div style={{ overflowX: 'auto', marginBottom: 14 }}>
        <Tabs
          variant="underline"
          active={service}
          onChange={onService}
          items={[{ id: ALL, label: t('logs.allServices') }, ...SERVICES.map(s => ({ id: s, label: s }))]}
        />
      </div>

      <div className="mf-row" style={{ marginBottom: 14 }}>
        <SearchInput
          className="mf-spacer"
          style={{ width: 250 }}
          placeholder={t('logs.filter')}
          value={q}
          onChange={e => setQ(e.target.value)}
        />
      </div>

      <AsyncView
        loading={loading}
        error={error}
        reload={load}
        empty={filtered.length === 0 ? (rows.length ? t('logs.emptyFilter') : t('logs.empty', { service })) : null}
      >
        <Card flush>
          {filtered.map((l, i) => (
            <div
              key={i}
              style={{
                display: 'grid',
                gridTemplateColumns: service === ALL ? '150px 66px 96px 90px 1fr' : '150px 66px 96px 1fr',
                gap: 14,
                alignItems: 'center',
                padding: '9px 16px',
                borderTop: i ? '1px solid var(--hair-soft)' : 'none',
              }}
            >
              <span className="mf-u-mono mf-u-faint" style={{ fontSize: 12 }}>{formatTime(l.time)}</span>
              <span><Pill tone={tone(severity(l.priority))}>{severity(l.priority)}</Pill></span>
              {service === ALL && (
                <span className="mf-u-mono mf-u-faint mf-truncate" style={{ fontSize: 11.5 }}>{l.service}</span>
              )}
              <span
                className="mf-u-mono mf-u-muted mf-truncate"
                style={{ fontSize: 11.5, background: 'var(--hair-soft)', padding: '2px 7px', borderRadius: 6, justifySelf: 'start', maxWidth: '100%' }}
              >
                {l.program || '—'}
              </span>
              <span className="mf-u-mono mf-truncate" style={{ fontSize: 12.5, color: 'var(--ink)' }}>{l.message}</span>
            </div>
          ))}
        </Card>
        {rows.length > 0 && (
          <div className="mf-u-faint" style={{ fontSize: 12, marginTop: 12 }}>
            {t('logs.linesShown', { count: filtered.length, total: rows.length })}
          </div>
        )}
      </AsyncView>
    </>
  );
}
