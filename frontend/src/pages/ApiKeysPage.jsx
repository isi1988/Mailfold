import React, { useState } from 'react';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { Table, TableRow } from '../ds/components/organisms/Table.jsx';
import { Pagination } from '../ds/components/molecules/Pagination.jsx';
import { Pill } from '../ds/components/atoms/Pill.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { ConfirmModal } from '../ds/components/organisms/ConfirmModal.jsx';
import { Empty } from '../components/States.jsx';
import { tone } from '../ds/data/sample.js';
import { useApi } from '../lib/useApi.js';
import { api } from '../api/client.js';
import { AsyncView } from '../components/States.jsx';
import { useToast } from '../components/Toast.jsx';
import { asList } from '../lib/format.js';
import { useT } from '../i18n/index.jsx';
import { ApiKeyDrawer } from './ApiKeyDrawer.jsx';

const PAGE_SIZE = 20;

// shortDate renders an ISO timestamp as a compact local date, or a dash.
function shortDate(v) {
  if (!v) return '—';
  const d = new Date(v);
  return isNaN(d) ? '—' : d.toLocaleDateString([], { year: 'numeric', month: 'short', day: 'numeric' });
}

export function ApiKeysPage() {
  const t = useT();
  const { toast } = useToast();
  const { data, loading, error, reload } = useApi('/api/apikeys', []);
  const [page, setPage] = useState(1);
  const [drawer, setDrawer] = useState(false);
  const [confirmKey, setConfirmKey] = useState(null);

  // The endpoint only exists when the subsystem is configured; a 404 means the
  // operator has not enabled API keys on this server, which we present calmly
  // rather than as an error.
  if (error && error.status === 404) {
    return (
      <>
        <PageHeader title={t('apikeys.title')} sub={t('apikeys.sub')} />
        <Empty message={t('apikeys.disabledTitle')}>
          <div className="mf-u-faint" style={{ fontSize: 13, marginTop: 6 }}>{t('apikeys.disabledHint')}</div>
        </Empty>
      </>
    );
  }

  async function doRevoke() {
    const k = confirmKey;
    setConfirmKey(null);
    try {
      await api.del('/api/apikeys/' + encodeURIComponent(k.id));
      toast(t('apikeys.revoked', { label: k.label || k.prefix }));
      reload();
    } catch (err) {
      toast(t('apikeys.failed'), (err && err.body && err.body.message) || (err && err.message) || '');
    }
  }

  const cols = [
    { label: t('apikeys.col.key'), w: '2fr' },
    { label: t('apikeys.col.mailbox'), w: '1.6fr' },
    { label: t('apikeys.col.scopes'), w: '1.6fr' },
    { label: t('apikeys.col.lastUsed'), w: '1fr' },
    { label: t('apikeys.col.status'), w: '.9fr' },
    { label: '', w: '18px' },
  ];

  const rows = asList(data);
  const pageCount = Math.max(1, Math.ceil(rows.length / PAGE_SIZE));
  const current = Math.min(page, pageCount);
  const paged = rows.slice((current - 1) * PAGE_SIZE, current * PAGE_SIZE);
  const from = rows.length === 0 ? 0 : (current - 1) * PAGE_SIZE + 1;
  const to = Math.min(current * PAGE_SIZE, rows.length);

  return (
    <>
      <PageHeader
        title={t('apikeys.title')}
        sub={t('apikeys.sub')}
        actions={<Button variant="primary" onClick={() => setDrawer(true)}>{t('apikeys.add')}</Button>}
      />

      <AsyncView
        loading={loading}
        error={error}
        reload={reload}
        empty={rows.length === 0 ? t('apikeys.empty') : null}
      >
        <Table columns={cols}>
          {paged.map(k => (
            <TableRow key={k.id} plain>
              <div className="mf-min0">
                <div className="mf-u-mono" style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>{k.prefix}…</div>
                {k.label ? <div className="mf-u-faint mf-truncate" style={{ fontSize: 11.5, marginTop: 2 }}>{k.label}</div> : null}
              </div>
              <span className="mf-u-muted mf-u-mono mf-truncate" style={{ fontSize: 12.5 }}>{k.mailbox}</span>
              <span className="mf-row" style={{ gap: 4, flexWrap: 'wrap' }}>
                {asList(k.scopes).map(s => <Pill key={s} tone="neutral">{t('apikeys.scope.' + s.replace('mail:', '')) || s}</Pill>)}
              </span>
              <span className="mf-u-muted" style={{ fontSize: 12.5 }}>{k.last_used ? shortDate(k.last_used) : t('apikeys.neverUsed')}</span>
              <span>
                <Pill tone={k.active ? tone('active') : 'neutral'}>
                  {k.active ? t('apikeys.active') : t('apikeys.revokedLabel')}
                </Pill>
              </span>
              {k.active
                ? <Button variant="ghost" size="sm" title={t('apikeys.revoke')} onClick={() => setConfirmKey(k)}><Icon name="trash" size={15} /></Button>
                : <span />}
            </TableRow>
          ))}
        </Table>
        {rows.length > 0 && (
          <div style={{ marginTop: 16 }}>
            <Pagination page={current} pageCount={pageCount} summary={t('common.showing', { from, to, total: rows.length })} onPage={setPage} />
          </div>
        )}
      </AsyncView>

      {drawer && (
        <ApiKeyDrawer onClose={() => setDrawer(false)} onSaved={reload} />
      )}
      {confirmKey && (
        <ConfirmModal
          title={t('apikeys.revokeTitle')}
          msg={t('apikeys.revokeMsg', { label: confirmKey.label || confirmKey.prefix })}
          cta={t('apikeys.revoke')}
          danger
          onCancel={() => setConfirmKey(null)}
          onConfirm={doRevoke}
        />
      )}
    </>
  );
}
