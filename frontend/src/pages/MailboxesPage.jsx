import React, { useState } from 'react';
import { useLocation } from 'react-router-dom';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { FilterTabs } from '../ds/components/molecules/FilterTabs.jsx';
import { SearchInput } from '../ds/components/molecules/SearchInput.jsx';
import { Table, TableRow } from '../ds/components/organisms/Table.jsx';
import { Pagination } from '../ds/components/molecules/Pagination.jsx';
import { Avatar } from '../ds/components/atoms/Avatar.jsx';
import { Pill } from '../ds/components/atoms/Pill.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { ProgressBar } from '../ds/components/atoms/ProgressBar.jsx';
import { ConfirmModal } from '../ds/components/organisms/ConfirmModal.jsx';
import { initials, tone } from '../ds/data/sample.js';
import { useApi } from '../lib/useApi.js';
import { api } from '../api/client.js';
import { AsyncView } from '../components/States.jsx';
import { useToast } from '../components/Toast.jsx';
import { isActive, human, asList } from '../lib/format.js';
import { useT } from '../i18n/index.jsx';
import { MailboxDrawer } from './MailboxDrawer.jsx';
import { MailboxImportDrawer } from './MailboxImportDrawer.jsx';

// Filter tabs keyed by a stable value with a translated label.
const TABS = [
  { value: 'All', key: 'all' },
  { value: 'Active', key: 'active' },
  { value: 'Disabled', key: 'disabled' },
];

const PAGE_SIZE = 20;

// lastLogin returns the most recent of the three protocol login timestamps
// (unix seconds), or 0 when the mailbox has never been used.
function lastLogin(m) {
  return Math.max(
    Number(m.last_imap_login) || 0,
    Number(m.last_smtp_login) || 0,
    Number(m.last_pop3_login) || 0,
  );
}

// clampPct keeps a usage percentage inside the 0–100 range for the bar width.
function clampPct(v) {
  const n = Number(v) || 0;
  return Math.max(0, Math.min(100, n));
}

// humanParts scales a byte count to the largest fitting unit, returning the
// numeric value and the unit so used/total can share one unit label.
function humanParts(bytes) {
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  let n = Number(bytes) || 0;
  let i = 0;
  while (n >= 1024 && i < units.length - 1) { n /= 1024; i += 1; }
  return { value: n, unit: units[i], i };
}

// num trims a scaled value to one decimal (or an integer when whole/large).
function num(v) {
  return v >= 10 || Number.isInteger(v) ? Math.round(v) : Math.round(v * 10) / 10;
}

// quotaLabel formats "used / total UNIT" with both sides in the total's unit and
// the unit shown once, e.g. "3.2 / 10 GB".
function quotaLabel(used, total) {
  const t = humanParts(total);
  const scale = Math.pow(1024, t.i);
  return num((Number(used) || 0) / scale) + ' / ' + num(t.value) + ' ' + t.unit;
}

// timeAgo renders a unix-second timestamp as a compact relative time, or an
// em-dash when the mailbox has never been used.
function timeAgo(ts, t) {
  if (!ts) return '—';
  const diff = Math.max(0, Math.floor(Date.now() / 1000) - ts);
  if (diff < 45) return t('mailboxes.time.now');
  if (diff < 3600) return t('mailboxes.time.min', { n: Math.floor(diff / 60) });
  if (diff < 86400) return t('mailboxes.time.hour', { n: Math.floor(diff / 3600) });
  if (diff < 2592000) return t('mailboxes.time.day', { n: Math.floor(diff / 86400) });
  if (diff < 31536000) return t('mailboxes.time.month', { n: Math.floor(diff / 2592000) });
  return t('mailboxes.time.year', { n: Math.floor(diff / 31536000) });
}

export function MailboxesPage() {
  const t = useT();
  const { toast } = useToast();
  const { data, loading, error, reload } = useApi('/api/mailboxes', []);
  const domainsApi = useApi('/api/domains', []);
  const [tab, setTab] = useState('All');
  const [q, setQ] = useState('');
  const [page, setPage] = useState(1);
  // A Link from the Dashboard's "+ New mailbox" action can pass state to open
  // the create drawer immediately, without a second click on this page.
  const location = useLocation();
  const [drawer, setDrawer] = useState(location.state && location.state.openCreate ? { mode: 'create' } : null); // { mode:'create' } | { mode:'edit', mailbox }
  const [confirmMb, setConfirmMb] = useState(null);
  const [importing, setImporting] = useState(false);

  async function doDelete() {
    const mb = confirmMb;
    setConfirmMb(null);
    try {
      await api.del('/api/mailboxes', { items: [mb.username] });
      toast(t('mailboxes.form.deleted', { mailbox: mb.username }));
      reload();
    } catch (err) {
      toast(t('mailboxes.form.failed'), (err && err.body && err.body.message) || (err && err.message) || '');
    }
  }

  const cols = [
    { label: t('mailboxes.col.mailbox'), w: '2.2fr' },
    { label: t('mailboxes.col.domain'), w: '1fr' },
    { label: t('mailboxes.col.quota'), w: '2fr' },
    { label: t('mailboxes.col.lastLogin'), w: '1fr' },
    { label: t('mailboxes.col.status'), w: '.9fr' },
    { label: '', w: '18px' },
  ];
  const tabOptions = TABS.map(x => ({ value: x.value, label: t('mailboxes.tabs.' + x.key) }));

  const rows = asList(data);
  const filtered = rows.filter(m => {
    if (tab === 'Active' && !isActive(m.active)) return false;
    if (tab === 'Disabled' && isActive(m.active)) return false;
    if (q) {
      const hay = ((m.username || '') + ' ' + (m.name || '')).toLowerCase();
      if (!hay.includes(q.toLowerCase())) return false;
    }
    return true;
  });

  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const current = Math.min(page, pageCount);
  const paged = filtered.slice((current - 1) * PAGE_SIZE, current * PAGE_SIZE);
  const from = filtered.length === 0 ? 0 : (current - 1) * PAGE_SIZE + 1;
  const to = Math.min(current * PAGE_SIZE, filtered.length);

  // Any change to the filter or query resets to the first page.
  const onTab = v => { setTab(v); setPage(1); };
  const onQuery = e => { setQ(e.target.value); setPage(1); };

  return (
    <>
      <PageHeader
        title={t('mailboxes.title')}
        sub={t('mailboxes.count', { count: rows.length })}
        actions={(
          <div className="mf-row" style={{ gap: 8 }}>
            <Button variant="secondary" onClick={() => setImporting(true)}>{t('mailboxes.importCsv')}</Button>
            <Button variant="primary" onClick={() => setDrawer({ mode: 'create' })}>{t('mailboxes.new')}</Button>
          </div>
        )}
      />
      <div className="mf-row" style={{ marginBottom: 14 }}>
        <FilterTabs options={tabOptions} value={tab} onSelect={onTab} />
        <SearchInput
          className="mf-spacer"
          style={{ width: 250 }}
          placeholder={t('mailboxes.filter')}
          value={q}
          onChange={onQuery}
        />
      </div>

      <AsyncView
        loading={loading}
        error={error}
        reload={reload}
        empty={filtered.length === 0 ? (rows.length ? t('mailboxes.emptyFilter') : t('mailboxes.empty')) : null}
      >
        <Table columns={cols}>
          {paged.map(m => (
            <TableRow key={m.username} onClick={() => setDrawer({ mode: 'edit', mailbox: m })} style={{ cursor: 'pointer' }}>
              <div className="mf-cell-user">
                <Avatar size={34}>{initials(m.name || m.username)}</Avatar>
                <div className="mf-min0">
                  <div className="mf-cell-name mf-truncate">{m.name || m.username}</div>
                  <div className="mf-cell-sub mf-truncate">{m.username}</div>
                </div>
              </div>
              <span className="mf-u-muted" style={{ fontSize: 13 }}>{m.domain}</span>
              {(() => {
                const pct = clampPct(m.percent_in_use);
                const unlimited = (Number(m.quota) || 0) <= 0;
                return (
                  <div style={{ minWidth: 0, paddingRight: 20 }}>
                    <div className="mf-row mf-row--between" style={{ marginBottom: 6, gap: 10 }}>
                      <span className="mf-u-faint mf-u-mono mf-truncate" style={{ fontSize: 12.5 }}>
                        {unlimited ? human(m.quota_used) + ' / ∞' : quotaLabel(m.quota_used, m.quota)}
                      </span>
                      {!unlimited && <span className="mf-u-faint mf-u-mono" style={{ fontSize: 12.5 }}>{Math.round(pct)}%</span>}
                    </div>
                    {!unlimited && (
                      <div aria-label={t('mailboxes.usage.label', { percent: Math.round(pct) })}>
                        <ProgressBar pct={pct} auto />
                      </div>
                    )}
                  </div>
                );
              })()}
              <span className="mf-u-muted" style={{ fontSize: 12.5 }}>{timeAgo(lastLogin(m), t)}</span>
              <span><Pill tone={isActive(m.active) ? tone('active') : 'neutral'}>{isActive(m.active) ? t('common.active') : t('common.disabled')}</Pill></span>
              <Icon name="chevron-right" size={14} style={{ color: 'var(--faint)' }} />
            </TableRow>
          ))}
        </Table>
        {filtered.length > 0 && (
          <div style={{ marginTop: 16 }}>
            <Pagination
              page={current}
              pageCount={pageCount}
              summary={t('mailboxes.showing', { from, to, total: filtered.length })}
              onPage={setPage}
            />
          </div>
        )}
      </AsyncView>

      {drawer && (
        <MailboxDrawer
          mode={drawer.mode}
          mailbox={drawer.mailbox}
          domains={asList(domainsApi.data)}
          onClose={() => setDrawer(null)}
          onSaved={reload}
          onDelete={mb => { setDrawer(null); setConfirmMb(mb); }}
        />
      )}

      {confirmMb && (
        <ConfirmModal
          title={t('mailboxes.form.deleteTitle')}
          msg={t('mailboxes.form.deleteMsg', { mailbox: confirmMb.username })}
          cta={t('common.delete')}
          danger
          onCancel={() => setConfirmMb(null)}
          onConfirm={doDelete}
        />
      )}

      {importing && (
        <MailboxImportDrawer onClose={() => setImporting(false)} onSaved={reload} />
      )}
    </>
  );
}
