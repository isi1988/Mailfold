import React, { useState } from 'react';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { SearchInput } from '../ds/components/molecules/SearchInput.jsx';
import { Tabs } from '../ds/components/molecules/Tabs.jsx';
import { Card } from '../ds/components/molecules/Card.jsx';
import { Pill } from '../ds/components/atoms/Pill.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { tone } from '../ds/data/sample.js';
import { useApi } from '../lib/useApi.js';
import { AsyncView } from '../components/States.jsx';
import { asList } from '../lib/format.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';

// Fixed set of mailcow log sources exposed by the /api/logs/{service} endpoint.
const SERVICES = ['postfix', 'dovecot', 'rspamd', 'sogo', 'netfilter'];

// Collapse an arbitrary syslog priority string into one of three buckets so it
// maps onto a Pill tone (info -> neutral, warn -> amber, error -> red).
function severity(priority) {
  const p = String(priority || '').toLowerCase();
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

export function LogsPage() {
  const t = useT();
  const { toast } = useToast();
  const [service, setService] = useState(SERVICES[0]);
  const [q, setQ] = useState('');
  const { data, loading, error, reload } = useApi('/api/logs/' + service, [service]);

  const rows = asList(data);
  const filtered = q
    ? rows.filter(l => {
        const hay = (
          (l.program || '') + ' ' + (l.priority || '') + ' ' + (l.message || '')
        ).toLowerCase();
        return hay.includes(q.toLowerCase());
      })
    : rows;

  // Switching the service tab refetches (deps: [service]) and clears the filter.
  const onService = id => { setService(id); setQ(''); };

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
        sub={t('logs.sub', { service })}
        actions={
          <>
            <Button variant="secondary" onClick={reload}>{t('logs.refresh')}</Button>
            <Button variant="secondary" onClick={download} disabled={rows.length === 0}>{t('logs.download')}</Button>
          </>
        }
      />

      <Tabs
        variant="underline"
        active={service}
        onChange={onService}
        items={SERVICES.map(s => ({ id: s, label: s }))}
        style={{ marginBottom: 14 }}
      />

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
        reload={reload}
        empty={filtered.length === 0 ? (rows.length ? t('logs.emptyFilter') : t('logs.empty', { service })) : null}
      >
        <Card flush>
          {filtered.map((l, i) => (
            <div
              key={i}
              style={{
                display: 'grid',
                gridTemplateColumns: '150px 66px 96px 1fr',
                gap: 14,
                alignItems: 'center',
                padding: '9px 16px',
                borderTop: i ? '1px solid var(--hair-soft)' : 'none',
              }}
            >
              <span className="mf-u-mono mf-u-faint" style={{ fontSize: 12 }}>{formatTime(l.time)}</span>
              <span><Pill tone={tone(severity(l.priority))}>{severity(l.priority)}</Pill></span>
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
