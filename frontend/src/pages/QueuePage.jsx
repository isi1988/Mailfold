import React, { useState } from 'react';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { SearchInput } from '../ds/components/molecules/SearchInput.jsx';
import { Table, TableRow } from '../ds/components/organisms/Table.jsx';
import { Pagination } from '../ds/components/molecules/Pagination.jsx';
import { Pill } from '../ds/components/atoms/Pill.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { ConfirmModal } from '../ds/components/organisms/ConfirmModal.jsx';
import { tone } from '../ds/data/sample.js';
import { useApi } from '../lib/useApi.js';
import { api } from '../api/client.js';
import { AsyncView } from '../components/States.jsx';
import { human, asList } from '../lib/format.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';
import { QueuePens } from './QueuePens.jsx';

const PAGE_SIZE = 20;

// mailcow's mailq payload shape varies between versions, so every field is read
// defensively from whatever key happens to be present, falling back to '-'.
function pick(row, keys) {
  for (const k of keys) {
    const v = row[k];
    if (v !== undefined && v !== null && v !== '') return v;
  }
  return undefined;
}

function fmtRecipients(row) {
  const v = pick(row, ['recipients', 'rcpt', 'recipient', 'recipient_address']);
  if (v === undefined) return '-';
  if (Array.isArray(v)) return v.length ? v.join(', ') : '-';
  return String(v);
}

function fmtSize(row) {
  const v = pick(row, ['message_size', 'size', 'message_size_bytes']);
  if (v === undefined) return '-';
  const n = Number(v);
  return Number.isFinite(n) ? human(n) : String(v);
}

// arrival_time may be a unix timestamp (seconds) or a preformatted string; when
// it is a timestamp we render a compact age like "12m" / "3h" / "2d".
function fmtAge(row) {
  const raw = pick(row, ['arrival_time', 'age', 'arrival', 'time']);
  if (raw === undefined) return '-';
  const ts = Number(raw);
  if (!Number.isFinite(ts) || ts <= 0) return String(raw);
  const secs = Math.max(0, Math.floor(Date.now() / 1000) - ts);
  if (secs < 60) return secs + 's';
  const mins = Math.floor(secs / 60);
  if (mins < 60) return mins + 'm';
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return hrs + 'h';
  return Math.floor(hrs / 24) + 'd';
}

function statusOf(row) {
  const v = pick(row, ['queue_name', 'status', 'queue']);
  return v === undefined ? '' : String(v);
}

function keyOf(row, i) {
  return pick(row, ['queue_id', 'id']) || String(i);
}

export function QueuePage() {
  const t = useT();
  const { toast } = useToast();
  const { data, loading, error, reload } = useApi('/api/queue', []);
  const [q, setQ] = useState('');
  const [page, setPage] = useState(1);
  const [confirmFlush, setConfirmFlush] = useState(false);
  const [confirmDeleteAll, setConfirmDeleteAll] = useState(false);
  const [busy, setBusy] = useState(false);

  async function doFlush() {
    setConfirmFlush(false);
    if (busy) return;
    setBusy(true);
    try {
      await api.post('/api/queue/flush', {});
      toast(t('queue.flush.done'));
      reload();
    } catch (err) {
      toast(t('queue.flush.failed'), (err && err.body && err.body.message) || (err && err.message) || '');
    } finally {
      setBusy(false);
    }
  }

  async function doDeleteAll() {
    setConfirmDeleteAll(false);
    if (busy) return;
    setBusy(true);
    try {
      await api.post('/api/queue/delete-all', {});
      toast(t('queue.deleteAll.done'));
      reload();
    } catch (err) {
      toast(t('queue.deleteAll.failed'), (err && err.body && err.body.message) || (err && err.message) || '');
    } finally {
      setBusy(false);
    }
  }

  const cols = [
    { label: t('queue.col.sender'), w: '1.5fr' },
    { label: t('queue.col.recipient'), w: '1.7fr' },
    { label: t('queue.col.size'), w: '.7fr' },
    { label: t('queue.col.age'), w: '.5fr' },
    { label: t('queue.col.status'), w: '.9fr' },
  ];

  const rows = asList(data);
  // Three broad buckets for the Cow-Managed pens: "active" is on its way out
  // right now, anything red-toned (reject/error/...) is stuck, everything
  // else (deferred, hold, unrecognized) is waiting its turn.
  const counts = rows.reduce((acc, r) => {
    const status = statusOf(r);
    if (status === 'active') acc.ready++;
    else if (tone(status) === 'red') acc.failed++;
    else acc.queued++;
    return acc;
  }, { ready: 0, queued: 0, failed: 0 });
  const filtered = q
    ? rows.filter(r => {
        const hay = (
          (pick(r, ['sender', 'from']) || '') + ' ' +
          fmtRecipients(r) + ' ' +
          statusOf(r)
        ).toLowerCase();
        return hay.includes(q.toLowerCase());
      })
    : rows;

  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const current = Math.min(page, pageCount);
  const paged = filtered.slice((current - 1) * PAGE_SIZE, current * PAGE_SIZE);
  const from = filtered.length === 0 ? 0 : (current - 1) * PAGE_SIZE + 1;
  const to = Math.min(current * PAGE_SIZE, filtered.length);
  const onQuery = e => { setQ(e.target.value); setPage(1); };

  return (
    <>
      <PageHeader
        title={t('queue.title')}
        sub={t('queue.count', { count: rows.length })}
        actions={
          <>
            <Button variant="danger" onClick={() => setConfirmDeleteAll(true)} disabled={busy || rows.length === 0}>
              {busy ? t('queue.deleteAll.busy') : t('queue.deleteAll.cta')}
            </Button>
            <Button variant="primary" onClick={() => setConfirmFlush(true)} disabled={busy || rows.length === 0}>
              {busy ? t('queue.flush.busy') : t('queue.flush.cta')}
            </Button>
          </>
        }
      />
      <QueuePens counts={counts} t={t} />
      <div className="mf-row" style={{ marginBottom: 14 }}>
        <SearchInput className="mf-spacer" style={{ width: 250 }} placeholder={t('queue.filter')} value={q} onChange={onQuery} />
      </div>

      <AsyncView
        loading={loading}
        error={error}
        reload={reload}
        empty={filtered.length === 0 ? (rows.length ? t('queue.emptyFilter') : t('queue.empty')) : null}
      >
        <Table columns={cols}>
          {paged.map((r, i) => {
            const sender = pick(r, ['sender', 'from']) || '-';
            const status = statusOf(r);
            return (
              <TableRow key={keyOf(r, (current - 1) * PAGE_SIZE + i)}>
                <span className="mf-truncate" style={{ fontSize: 13, fontWeight: 500, color: 'var(--ink)' }}>{sender}</span>
                <span className="mf-u-muted mf-truncate" style={{ fontSize: 13 }}>{fmtRecipients(r)}</span>
                <span className="mf-u-mono mf-u-muted" style={{ fontSize: 12.5 }}>{fmtSize(r)}</span>
                <span className="mf-u-mono mf-u-faint" style={{ fontSize: 12.5 }}>{fmtAge(r)}</span>
                <span>{status ? <Pill tone={tone(status)}>{status}</Pill> : <span className="mf-u-faint">-</span>}</span>
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

      {confirmFlush && (
        <ConfirmModal
          title={t('queue.flush.confirmTitle')}
          msg={t('queue.flush.confirmMsg')}
          cta={t('queue.flush.cta')}
          danger
          onCancel={() => setConfirmFlush(false)}
          onConfirm={doFlush}
        />
      )}
      {confirmDeleteAll && (
        <ConfirmModal
          title={t('queue.deleteAll.confirmTitle')}
          msg={t('queue.deleteAll.confirmMsg')}
          cta={t('queue.deleteAll.cta')}
          danger
          onCancel={() => setConfirmDeleteAll(false)}
          onConfirm={doDeleteAll}
        />
      )}
    </>
  );
}
