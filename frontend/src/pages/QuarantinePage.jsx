import React, { useState } from 'react';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { SearchInput } from '../ds/components/molecules/SearchInput.jsx';
import { Table, TableRow } from '../ds/components/organisms/Table.jsx';
import { Pagination } from '../ds/components/molecules/Pagination.jsx';
import { Pill } from '../ds/components/atoms/Pill.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { ConfirmModal } from '../ds/components/organisms/ConfirmModal.jsx';
import { useApi } from '../lib/useApi.js';
import { api } from '../api/client.js';
import { AsyncView } from '../components/States.jsx';
import { asList } from '../lib/format.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';

const PAGE_SIZE = 20;

// Quarantined items come straight from mailcow, whose fields drift by version.
// Everything is read defensively with several fallbacks.
function rowId(q) {
  return q.id ?? q.qid ?? q.rspamd_hash ?? q.rspamd_qid ?? null;
}
function sender(q) {
  return q.sender || q.from || q.header_from || q.env_from || '';
}
function recipient(q) {
  return q.rcpt || q.recipient || q.rcpts || q.env_rcpt || '';
}
function subject(q) {
  return q.subject || '';
}
function reason(q) {
  return q.action || q.reason || q.qType || q.type || '';
}
function scoreOf(q) {
  const raw = q.score ?? q.spam_score ?? q.rspamd_score;
  const n = Number(raw);
  return Number.isFinite(n) ? n : null;
}

// mailcow stores the hold time as a unix timestamp (seconds); fall back to any
// string date it might hand back instead.
function heldAt(q) {
  const raw = q.created || q.timestamp || q.date;
  if (raw == null || raw === '') return '';
  const n = Number(raw);
  const d = Number.isFinite(n) && n > 0 ? new Date(n * 1000) : new Date(raw);
  if (Number.isNaN(d.getTime())) return typeof raw === 'string' ? raw : '';
  return d.toLocaleString();
}

function scoreTone(score) {
  if (score == null) return 'neutral';
  if (score >= 15) return 'red';
  if (score >= 8) return 'amber';
  return 'blue';
}

export function QuarantinePage() {
  const t = useT();
  const { toast } = useToast();
  const { data, loading, error, reload } = useApi('/api/quarantine', []);
  const [q, setQ] = useState('');
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState([]); // ids
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [busy, setBusy] = useState(false);

  const cols = [
    { label: '', w: '24px' },
    { label: t('quarantine.col.subject'), w: '2.1fr' },
    { label: t('quarantine.col.recipient'), w: '1.3fr' },
    { label: t('quarantine.col.score'), w: '.55fr' },
    { label: t('quarantine.col.reason'), w: '.9fr' },
    { label: t('quarantine.col.held'), w: '.9fr' },
  ];

  const rows = asList(data);
  const filtered = q
    ? rows.filter(r => {
        const hay = (sender(r) + ' ' + recipient(r) + ' ' + subject(r) + ' ' + reason(r)).toLowerCase();
        return hay.includes(q.toLowerCase());
      })
    : rows;

  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const current = Math.min(page, pageCount);
  const paged = filtered.slice((current - 1) * PAGE_SIZE, current * PAGE_SIZE);
  const from = filtered.length === 0 ? 0 : (current - 1) * PAGE_SIZE + 1;
  const to = Math.min(current * PAGE_SIZE, filtered.length);
  const onQuery = e => { setQ(e.target.value); setPage(1); };

  // Ids selectable on the current page (rows without an id can't be acted on).
  const pageIds = paged.map(rowId).filter(id => id != null).map(String);
  const allOnPage = pageIds.length > 0 && pageIds.every(id => selected.includes(id));
  const selectedCount = selected.length;

  function toggle(id) {
    const key = String(id);
    setSelected(cur => (cur.includes(key) ? cur.filter(x => x !== key) : [...cur, key]));
  }
  function toggleAll() {
    if (allOnPage) setSelected(cur => cur.filter(id => !pageIds.includes(id)));
    else setSelected(cur => Array.from(new Set([...cur, ...pageIds])));
  }

  async function doDelete() {
    if (busy) return;
    setBusy(true);
    const items = selected.slice();
    try {
      await api.del('/api/quarantine', { items });
      toast(t('quarantine.deleted', { count: items.length }));
      setSelected([]);
      setConfirmDelete(false);
      reload();
    } catch (err) {
      toast(t('quarantine.failed'), (err && err.body && err.body.message) || (err && err.message) || '');
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <PageHeader
        title={t('quarantine.title')}
        sub={t('quarantine.count', { count: rows.length })}
        actions={
          <>
            {/* mailcow exposes no release/whitelist endpoint — shown disabled to match the design. */}
            <Button variant="secondary" disabled title={t('quarantine.releaseHint')}>{t('quarantine.release')}</Button>
            <Button variant="danger" disabled={selectedCount === 0} onClick={() => setConfirmDelete(true)}>
              {selectedCount > 0 ? t('quarantine.deleteN', { count: selectedCount }) : t('quarantine.delete')}
            </Button>
          </>
        }
      />
      <div className="mf-row" style={{ marginBottom: 14 }}>
        <SearchInput className="mf-spacer" style={{ width: 250 }} placeholder={t('quarantine.filter')} value={q} onChange={onQuery} />
      </div>

      <AsyncView
        loading={loading}
        error={error}
        reload={reload}
        empty={filtered.length === 0 ? (rows.length ? t('quarantine.emptyFilter') : t('quarantine.empty')) : null}
      >
        <Table columns={cols}>
          <TableRow plain>
            <input
              type="checkbox"
              aria-label={t('quarantine.selectAll')}
              checked={allOnPage}
              onChange={toggleAll}
              disabled={pageIds.length === 0}
              style={{ cursor: pageIds.length === 0 ? 'default' : 'pointer' }}
            />
            <span className="mf-u-faint" style={{ fontSize: 11.5 }}>{t('quarantine.selectAll')}</span>
            <span /><span /><span /><span />
          </TableRow>
          {paged.map((r, i) => {
            const id = rowId(r);
            const key = id != null ? String(id) : null;
            const score = scoreOf(r);
            const rsn = reason(r);
            return (
              <TableRow key={key || i}>
                <input
                  type="checkbox"
                  aria-label={t('quarantine.selectRow')}
                  checked={key != null && selected.includes(key)}
                  onChange={() => key != null && toggle(key)}
                  disabled={key == null}
                  style={{ cursor: key == null ? 'default' : 'pointer' }}
                />
                <div className="mf-min0">
                  <div className="mf-truncate" style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>
                    {subject(r) || t('quarantine.noSubject')}
                  </div>
                  <div className="mf-u-faint mf-u-mono mf-truncate" style={{ fontSize: 11.5 }}>{sender(r)}</div>
                </div>
                <span className="mf-u-muted mf-u-mono mf-truncate" style={{ fontSize: 12.5 }}>{recipient(r)}</span>
                <span className="mf-u-mono" style={{ fontSize: 13, fontWeight: 600, color: 'var(--' + scoreTone(score) + ')' }}>
                  {score == null ? '—' : score}
                </span>
                <span>{rsn ? <Pill tone="amber">{rsn}</Pill> : <span className="mf-u-faint">—</span>}</span>
                <span className="mf-u-faint" style={{ fontSize: 12 }}>{heldAt(r) || '—'}</span>
              </TableRow>
            );
          })}
        </Table>
        {filtered.length > 0 && (
          <div style={{ marginTop: 16 }}>
            <Pagination page={current} pageCount={pageCount} summary={t('common.showing', { from, to, total: filtered.length })} onPage={setPage} />
          </div>
        )}
      </AsyncView>

      {confirmDelete && (
        <ConfirmModal
          title={t('quarantine.deleteTitle')}
          msg={t('quarantine.deleteMsg', { count: selectedCount })}
          cta={busy ? t('quarantine.deleting') : t('common.delete')}
          danger
          onCancel={() => (busy ? null : setConfirmDelete(false))}
          onConfirm={doDelete}
        />
      )}
    </>
  );
}
