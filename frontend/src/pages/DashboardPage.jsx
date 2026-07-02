import React from 'react';
import { Link } from 'react-router-dom';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { StatCard } from '../ds/components/molecules/StatCard.jsx';
import { Card } from '../ds/components/molecules/Card.jsx';
import { StatusDot } from '../ds/components/atoms/StatusDot.jsx';
import { ProgressBar } from '../ds/components/atoms/ProgressBar.jsx';
import { useApi } from '../lib/useApi.js';
import { AsyncView } from '../components/States.jsx';
import { asList } from '../lib/format.js';
import { useT } from '../i18n/index.jsx';

// mailcow reports used_percent either as a number (45) or a string ("45" / "45%").
function pctOf(v) {
  const n = Number(String(v == null ? '' : v).replace('%', '').trim());
  if (!Number.isFinite(n)) return 0;
  return Math.max(0, Math.min(100, Math.round(n)));
}

// The vmail payload may arrive as {} (or null) on an upstream hiccup; read fields
// defensively and fall back to a dash for the human-readable size strings.
function vmailInfo(data) {
  const v = data && typeof data === 'object' && !Array.isArray(data) ? data : {};
  return {
    used: v.used != null && v.used !== '' ? String(v.used) : '—',
    total: v.total != null && v.total !== '' ? String(v.total) : '',
    pct: pctOf(v.used_percent),
  };
}

// containers is an object keyed by container name → { state, image, started_at }.
// Tolerate {}/null/array and normalise to a sorted list of rows.
function containerList(data) {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return [];
  return Object.keys(data)
    .map(name => {
      const c = data[name] || {};
      return { name, state: c.state || '', image: c.image || '' };
    })
    .sort((a, b) => a.name.localeCompare(b.name));
}

export function DashboardPage() {
  const t = useT();
  const version = useApi('/api/status/version', []);
  const vmail = useApi('/api/status/vmail', []);
  const containers = useApi('/api/status/containers', []);
  const mailboxes = useApi('/api/mailboxes', []);
  const domains = useApi('/api/domains', []);
  const queue = useApi('/api/queue', []);

  // The overview is a fan-out of read-only status calls; treat the whole page as
  // loading until every source settles, and surface the first error with retry.
  const sources = [version, vmail, containers, mailboxes, domains, queue];
  const loading = sources.some(s => s.loading);
  const error = sources.map(s => s.error).find(Boolean) || null;
  const reload = () => sources.forEach(s => s.reload());

  const ver =
    version.data && typeof version.data === 'object' && !Array.isArray(version.data)
      ? version.data.version
      : null;
  const store = vmailInfo(vmail.data);
  const services = containerList(containers.data);
  const running = services.filter(s => s.state === 'running').length;

  const mailboxCount = asList(mailboxes.data).length;
  const domainCount = asList(domains.data).length;
  const queueCount = asList(queue.data).length;

  return (
    <>
      <PageHeader
        title={t('dashboard.title')}
        sub={ver ? t('dashboard.subVersion', { version: ver }) : t('dashboard.sub')}
      />

      <AsyncView loading={loading} error={error} reload={reload}>
        {/* KPI row — each card links to its section */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(160px,1fr))', gap: 16, marginBottom: 16 }}>
          <Link to="/mailboxes" style={{ textDecoration: 'none', color: 'inherit' }}>
            <StatCard size="lg" label={t('dashboard.kpi.mailboxes')} value={mailboxCount} delta={t('dashboard.kpi.view')} deltaTone="muted" />
          </Link>
          <Link to="/domains" style={{ textDecoration: 'none', color: 'inherit' }}>
            <StatCard size="lg" label={t('dashboard.kpi.domains')} value={domainCount} delta={t('dashboard.kpi.view')} deltaTone="muted" />
          </Link>
          <StatCard
            size="lg"
            label={t('dashboard.kpi.storage')}
            value={store.used}
            pct={store.pct}
            note={store.total ? t('dashboard.kpi.storageNote', { pct: store.pct, total: store.total }) : t('dashboard.kpi.storagePctOnly', { pct: store.pct })}
          />
          <Link to="/queue" style={{ textDecoration: 'none', color: 'inherit' }}>
            <StatCard
              size="lg"
              label={t('dashboard.kpi.queue')}
              value={queueCount}
              delta={queueCount > 0 ? t('dashboard.kpi.queueBacklog') : t('dashboard.kpi.queueClear')}
              deltaTone={queueCount > 0 ? 'amber' : 'green'}
              dot
            />
          </Link>
        </div>

        {/* Service health + storage */}
        <div style={{ display: 'grid', gridTemplateColumns: '1.6fr 1fr', gap: 16 }}>
          <Card pad>
            <div className="mf-row" style={{ marginBottom: 6 }}>
              <span className="mf-card__title">{t('dashboard.health.title')}</span>
              <span className="mf-spacer mf-row mf-u-muted" style={{ gap: 7, fontSize: 12 }}>
                <StatusDot tone={services.length && running === services.length ? 'green' : 'amber'} />
                {t('dashboard.health.summary', { running, total: services.length })}
              </span>
            </div>
            {services.length === 0 ? (
              <div className="mf-u-muted" style={{ fontSize: 13, padding: '10px 0' }}>{t('dashboard.health.empty')}</div>
            ) : (
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0 24px' }}>
                {services.map(s => {
                  const up = s.state === 'running';
                  return (
                    <div key={s.name} className="mf-row" style={{ gap: 9, padding: '7px 0', borderTop: '1px solid var(--hair-soft)' }}>
                      <StatusDot tone={up ? 'green' : 'red'} />
                      <span className="mf-u-mono" style={{ fontSize: 12.5, color: 'var(--ink)' }}>{s.name}</span>
                      <span className="mf-spacer mf-u-mono" style={{ fontSize: 11.5, color: up ? 'var(--faint)' : 'var(--red)' }}>
                        {up ? t('dashboard.health.running') : (s.state || t('dashboard.health.down'))}
                      </span>
                    </div>
                  );
                })}
              </div>
            )}
          </Card>

          <Card pad style={{ display: 'flex', flexDirection: 'column' }}>
            <div className="mf-row">
              <span className="mf-card__title">{t('dashboard.storage.title')}</span>
              <span className="mf-spacer mf-u-faint" style={{ fontSize: 11.5 }}>{t('dashboard.storage.vmail')}</span>
            </div>
            <div style={{ fontFamily: 'var(--font-serif)', fontSize: 24, color: 'var(--ink-strong)', margin: '10px 0 4px' }}>
              {store.used}
              {store.total ? (
                <span style={{ fontSize: 12, fontFamily: 'var(--font-sans)', color: 'var(--muted)' }}> / {store.total}</span>
              ) : null}
            </div>
            <div className="mf-u-muted" style={{ fontSize: 12, marginBottom: 12 }}>
              {t('dashboard.storage.usedPct', { pct: store.pct })}
            </div>
            <div style={{ marginTop: 'auto' }}>
              <ProgressBar pct={store.pct} auto lg />
            </div>
          </Card>
        </div>
      </AsyncView>
    </>
  );
}
