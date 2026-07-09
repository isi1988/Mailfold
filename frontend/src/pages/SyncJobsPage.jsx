import React, { useState } from 'react';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { SearchInput } from '../ds/components/molecules/SearchInput.jsx';
import { Table, TableRow } from '../ds/components/organisms/Table.jsx';
import { Pagination } from '../ds/components/molecules/Pagination.jsx';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Toggle } from '../ds/components/atoms/Toggle.jsx';
import { Pill } from '../ds/components/atoms/Pill.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { ConfirmModal } from '../ds/components/organisms/ConfirmModal.jsx';
import { tone } from '../ds/data/sample.js';
import { useApi } from '../lib/useApi.js';
import { api } from '../api/client.js';
import { AsyncView } from '../components/States.jsx';
import { useToast } from '../components/Toast.jsx';
import { isActive, asList } from '../lib/format.js';
import { decodeIdnAddress } from '../lib/idn.js';
import { useT } from '../i18n/index.jsx';

const PAGE_SIZE = 20;
const ENC_OPTIONS = ['SSL', 'TLS', 'PLAIN'];

// A sync job's live status is derived from three defensive mailcow fields:
// is_running trumps everything, then active toggles idle vs paused. mailcow does
// not expose an explicit error flag on the list, so we surface running/idle/paused.
function jobStatus(job) {
  if (isActive(job.is_running)) return 'running';
  if (!isActive(job.active)) return 'paused';
  return 'idle';
}

// last_run may be a datetime string, 0/"0", or empty when the job never ran.
function lastRun(job, never) {
  const v = job.last_run;
  if (!v || v === '0' || v === 0 || String(v).startsWith('0000')) return never;
  return String(v);
}

function errText(err, fallback) {
  return (err && err.body && err.body.message) || (err && err.message) || fallback;
}

export function SyncJobsPage() {
  const t = useT();
  const { toast } = useToast();
  const { data, loading, error, reload } = useApi('/api/syncjobs', []);
  const mailboxesApi = useApi('/api/mailboxes', []);
  const [q, setQ] = useState('');
  const [page, setPage] = useState(1);
  const [drawer, setDrawer] = useState(null); // { mode:'create' } | { mode:'edit', job }
  const [confirmJob, setConfirmJob] = useState(null);

  async function doDelete() {
    const job = confirmJob;
    setConfirmJob(null);
    try {
      await api.del('/api/syncjobs', { items: [String(job.id)] });
      toast(t('syncjobs.form.deleted', { job: decodeIdnAddress(job.user1 || job.host1 || job.id) }));
      reload();
    } catch (err) {
      toast(t('syncjobs.form.failed'), errText(err, ''));
    }
  }

  const statusLabel = { running: t('syncjobs.status.running'), idle: t('syncjobs.status.idle'), paused: t('syncjobs.status.paused') };

  const cols = [
    { label: t('syncjobs.col.job'), w: '1.8fr' },
    { label: t('syncjobs.col.target'), w: '1.4fr' },
    { label: t('syncjobs.col.every'), w: '.7fr' },
    { label: t('syncjobs.col.lastRun'), w: '.9fr' },
    { label: t('syncjobs.col.status'), w: '.9fr' },
    { label: '', w: '18px' },
  ];

  const rows = asList(data);
  const filtered = q
    ? rows.filter(s => {
        const hay = ((s.user1 || '') + ' ' + (s.host1 || '') + ' ' + (s.user2 || '')).toLowerCase();
        return hay.includes(q.toLowerCase());
      })
    : rows;

  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const current = Math.min(page, pageCount);
  const paged = filtered.slice((current - 1) * PAGE_SIZE, current * PAGE_SIZE);
  const from = filtered.length === 0 ? 0 : (current - 1) * PAGE_SIZE + 1;
  const to = Math.min(current * PAGE_SIZE, filtered.length);
  const onQuery = e => { setQ(e.target.value); setPage(1); };

  const empty = filtered.length === 0
    ? (rows.length
        ? t('syncjobs.emptyFilter')
        : <EmptyCta t={t} onCreate={() => setDrawer({ mode: 'create' })} />)
    : null;

  return (
    <>
      <PageHeader
        title={t('syncjobs.title')}
        sub={t('syncjobs.count', { count: rows.length })}
        actions={<Button variant="primary" onClick={() => setDrawer({ mode: 'create' })}>{t('syncjobs.new')}</Button>}
      />
      <div className="mf-row" style={{ marginBottom: 14 }}>
        <SearchInput className="mf-spacer" style={{ width: 250 }} placeholder={t('syncjobs.filter')} value={q} onChange={onQuery} />
      </div>

      <AsyncView loading={loading} error={error} reload={reload} empty={empty}>
        <Table columns={cols}>
          {paged.map(s => {
            const st = jobStatus(s);
            const source = decodeIdnAddress(s.host1 || '') + (s.user1 ? (s.host1 ? ' · ' : '') + decodeIdnAddress(s.user1) : '');
            return (
              <TableRow key={s.id} onClick={() => setDrawer({ mode: 'edit', job: s })} style={{ cursor: 'pointer' }}>
                <div className="mf-min0">
                  <div className="mf-truncate" style={{ fontSize: 13.5, fontWeight: 600, color: 'var(--ink)' }}>{decodeIdnAddress(s.user1 || s.host1 || t('syncjobs.unnamed'))}</div>
                  <div className="mf-u-faint mf-u-mono mf-truncate" style={{ fontSize: 11.5 }}>{source || '—'}</div>
                </div>
                <span className="mf-u-muted mf-u-mono mf-truncate" style={{ fontSize: 12.5 }}>{decodeIdnAddress(s.user2) || '—'}</span>
                <span className="mf-u-muted" style={{ fontSize: 12.5 }}>{t('syncjobs.everyMin', { count: Number(s.mins_interval) || 0 })}</span>
                <span className="mf-u-faint" style={{ fontSize: 12.5 }}>{lastRun(s, t('syncjobs.never'))}</span>
                <span><Pill tone={tone(st)}>{statusLabel[st]}</Pill></span>
                <Icon name="chevron-right" size={14} style={{ color: 'var(--faint)' }} />
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

      {drawer && (
        <SyncJobDrawer
          mode={drawer.mode}
          job={drawer.job}
          mailboxes={asList(mailboxesApi.data)}
          onClose={() => setDrawer(null)}
          onSaved={reload}
          onDelete={job => { setDrawer(null); setConfirmJob(job); }}
        />
      )}

      {confirmJob && (
        <ConfirmModal
          title={t('syncjobs.form.deleteTitle')}
          msg={t('syncjobs.form.deleteMsg', { job: decodeIdnAddress(confirmJob.user1 || confirmJob.host1 || confirmJob.id) })}
          cta={t('common.delete')}
          danger
          onCancel={() => setConfirmJob(null)}
          onConfirm={doDelete}
        />
      )}
    </>
  );
}

// Empty-state block with a call to action to create the first sync job.
function EmptyCta({ t, onCreate }) {
  return (
    <div style={{ padding: '56px 12px', textAlign: 'center' }} className="mf-u-muted">
      <div style={{ fontSize: 14 }}>{t('syncjobs.empty')}</div>
      <div style={{ marginTop: 14 }}>
        <Button variant="primary" size="sm" onClick={onCreate}>{t('syncjobs.new')}</Button>
      </div>
    </div>
  );
}

/**
 * Create / edit an IMAP sync job in a right-hand slide-over.
 *   mode        'create' | 'edit'
 *   job         the row (edit mode)
 *   mailboxes   [{username}] for the target-mailbox picker
 *   onClose     () => void
 *   onSaved     () => void  — parent refetches the list
 *   onDelete    (job) => void
 */
function SyncJobDrawer({ mode, job, mailboxes = [], onClose, onSaved, onDelete }) {
  const t = useT();
  const { toast } = useToast();
  const editing = mode === 'edit';

  const [host1, setHost1] = useState(editing ? job.host1 || '' : '');
  const [port1, setPort1] = useState(editing ? String(job.port1 || '993') : '993');
  const [user1, setUser1] = useState(editing ? job.user1 || '' : '');
  const [password1, setPassword1] = useState('');
  const [enc1, setEnc1] = useState(editing ? (job.enc1 || 'SSL') : 'SSL');
  const [user2, setUser2] = useState(editing ? (job.user2 || '') : (mailboxes[0] ? mailboxes[0].username : ''));
  const [minsInterval, setMinsInterval] = useState(editing ? String(job.mins_interval || '20') : '20');
  const [active, setActive] = useState(editing ? isActive(job.active) : true);
  const [deleteSource, setDeleteSource] = useState(editing ? isActive(job.delete1) : false);
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (busy) return;
    if (!editing && (!host1.trim() || !user1.trim() || password1.length < 1 || !user2)) {
      toast(t('syncjobs.form.invalid'));
      return;
    }
    setBusy(true);
    try {
      if (editing) {
        const attr = {
          host1: host1.trim(),
          port1: Number(port1) || 993,
          user1: user1.trim(),
          enc1,
          user2,
          mins_interval: Number(minsInterval) || 20,
          active: active ? '1' : '0',
          delete1: deleteSource ? '1' : '0',
        };
        if (password1) attr.password1 = password1;
        await api.put('/api/syncjobs', { items: [String(job.id)], attr });
        toast(t('syncjobs.form.updated', { job: decodeIdnAddress(user1.trim() || host1.trim()) }));
      } else {
        await api.post('/api/syncjobs', {
          host1: host1.trim(),
          port1: Number(port1) || 993,
          user1: user1.trim(),
          password1,
          enc1,
          mins_interval: Number(minsInterval) || 20,
          user2,
          active: active ? '1' : '0',
          delete1: deleteSource ? '1' : '0',
        });
        toast(t('syncjobs.form.created', { job: decodeIdnAddress(user1.trim()) }));
      }
      onSaved();
      onClose();
    } catch (err) {
      toast(t('syncjobs.form.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  const footer = (
    <>
      {editing && onDelete && (
        <Button variant="danger" onClick={() => onDelete(job)}>{t('common.delete')}</Button>
      )}
      <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
      <Button variant="primary" onClick={submit} disabled={busy}>
        {busy ? t('syncjobs.form.saving') : (editing ? t('common.save') : t('syncjobs.form.create'))}
      </Button>
    </>
  );

  return (
    <Drawer
      title={editing ? t('syncjobs.form.editTitle') : t('syncjobs.form.newTitle')}
      subtitle={decodeIdnAddress(user1 || host1) || t('syncjobs.form.newTitle')}
      footer={footer}
      onClose={onClose}
    >
      <div className="mf-row" style={{ gap: 10 }}>
        <FormField label={t('syncjobs.form.host')} style={{ flex: 2 }}>
          <Input placeholder="imap.gmail.com" value={host1} onChange={e => setHost1(e.target.value)} />
        </FormField>
        <FormField label={t('syncjobs.form.port')} style={{ flex: 1 }}>
          <Input type="number" min="1" align="right" value={port1} onChange={e => setPort1(e.target.value)} />
        </FormField>
      </div>

      <FormField label={t('syncjobs.form.encryption')}>
        <select className="mf-input" value={enc1} onChange={e => setEnc1(e.target.value)}>
          {ENC_OPTIONS.map(o => <option key={o} value={o}>{o}</option>)}
        </select>
      </FormField>

      <FormField label={t('syncjobs.form.user')}>
        <Input placeholder="jane@gmail.com" value={user1} onChange={e => setUser1(e.target.value)} />
      </FormField>

      <FormField label={editing ? t('syncjobs.form.resetPassword') : t('syncjobs.form.password')}>
        <Input
          type="text"
          mono
          placeholder={editing ? t('syncjobs.form.leaveBlank') : '••••••••'}
          value={password1}
          onChange={e => setPassword1(e.target.value)}
        />
      </FormField>

      <FormField label={t('syncjobs.form.target')}>
        {mailboxes.length ? (
          <select className="mf-input" value={user2} onChange={e => setUser2(e.target.value)}>
            {!editing && !user2 && <option value="" disabled>{t('syncjobs.form.selectTarget')}</option>}
            {mailboxes.map(m => <option key={m.username} value={m.username}>{decodeIdnAddress(m.username)}</option>)}
            {editing && user2 && !mailboxes.some(m => m.username === user2) && (
              <option value={user2}>{decodeIdnAddress(user2)}</option>
            )}
          </select>
        ) : (
          <Input placeholder="jane@acme.io" value={user2} onChange={e => setUser2(e.target.value)} />
        )}
      </FormField>

      <FormField label={t('syncjobs.form.interval')}>
        <Input type="number" min="1" align="right" value={minsInterval} onChange={e => setMinsInterval(e.target.value)} />
      </FormField>

      <div className="mf-row mf-row--between" style={{ marginTop: 6 }}>
        <span>
          <span className="mf-u-muted" style={{ fontSize: 13 }}>{t('syncjobs.form.deleteSource')}</span>
          <div className="mf-u-faint" style={{ fontSize: 11.5, marginTop: 2 }}>{t('syncjobs.form.deleteSourceHint')}</div>
        </span>
        <Toggle on={deleteSource} onClick={() => setDeleteSource(v => !v)} style={{ cursor: 'pointer', flex: 'none' }} />
      </div>

      <div className="mf-row mf-row--between" style={{ marginTop: 14 }}>
        <span className="mf-u-muted" style={{ fontSize: 13 }}>{t('syncjobs.form.active')}</span>
        <Toggle on={active} onClick={() => setActive(a => !a)} style={{ cursor: 'pointer' }} />
      </div>
    </Drawer>
  );
}
