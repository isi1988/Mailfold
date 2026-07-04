import React, { useState } from 'react';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { SearchInput } from '../ds/components/molecules/SearchInput.jsx';
import { Table, TableRow } from '../ds/components/organisms/Table.jsx';
import { Pagination } from '../ds/components/molecules/Pagination.jsx';
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
import { AliasDrawer } from './AliasDrawer.jsx';

const PAGE_SIZE = 20;

export function AliasesPage() {
  const t = useT();
  const { toast } = useToast();
  const { data, loading, error, reload } = useApi('/api/aliases', []);
  const domainsApi = useApi('/api/domains', []);
  const mailboxesApi = useApi('/api/mailboxes', []);
  const [q, setQ] = useState('');
  const [page, setPage] = useState(1);
  const [drawer, setDrawer] = useState(null);
  const [confirmAlias, setConfirmAlias] = useState(null);

  async function doDelete() {
    const a = confirmAlias;
    setConfirmAlias(null);
    try {
      await api.del('/api/aliases', { items: [String(a.id)] });
      toast(t('aliases.form.deleted', { alias: decodeIdnAddress(a.address) }));
      reload();
    } catch (err) {
      toast(t('aliases.form.failed'), (err && err.body && err.body.message) || (err && err.message) || '');
    }
  }

  const cols = [
    { label: t('aliases.col.alias'), w: '1.6fr' },
    { label: t('aliases.col.forwards'), w: '2fr' },
    { label: t('aliases.col.status'), w: '.9fr' },
    { label: '', w: '18px' },
  ];

  const rows = asList(data);
  const filtered = q
    ? rows.filter(a => ((a.address || '') + ' ' + (a.goto || '')).toLowerCase().includes(q.toLowerCase()))
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
        title={t('aliases.title')}
        sub={t('aliases.count', { count: rows.length })}
        actions={<Button variant="primary" onClick={() => setDrawer({ mode: 'create' })}>{t('aliases.add')}</Button>}
      />
      <div className="mf-row" style={{ marginBottom: 14 }}>
        <SearchInput className="mf-spacer" style={{ width: 250 }} placeholder={t('aliases.filter')} value={q} onChange={onQuery} />
      </div>

      <AsyncView
        loading={loading}
        error={error}
        reload={reload}
        empty={filtered.length === 0 ? (rows.length ? t('aliases.emptyFilter') : t('aliases.empty')) : null}
      >
        <Table columns={cols}>
          {paged.map(a => (
            <TableRow key={a.address} onClick={() => setDrawer({ mode: 'edit', alias: a })} style={{ cursor: 'pointer' }}>
              <span className="mf-u-mono mf-truncate" style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>{decodeIdnAddress(a.address)}</span>
              <div className="mf-row mf-min0" style={{ gap: 8 }}>
                <Icon name="arrow-right" size={14} style={{ color: 'var(--faint)', flex: 'none' }} />
                <span className="mf-u-muted mf-truncate" style={{ fontSize: 13 }}>{(a.goto || '').split(',').map(decodeIdnAddress).join(', ')}</span>
              </div>
              <span><Pill tone={isActive(a.active) ? tone('active') : 'neutral'}>{isActive(a.active) ? t('common.active') : t('common.inactive')}</Pill></span>
              <Icon name="chevron-right" size={14} style={{ color: 'var(--faint)' }} />
            </TableRow>
          ))}
        </Table>
        {filtered.length > 0 && (
          <div style={{ marginTop: 16 }}>
            <Pagination page={current} pageCount={pageCount} summary={t('common.showing', { from, to, total: filtered.length })} onPage={setPage} />
          </div>
        )}
      </AsyncView>

      {drawer && (
        <AliasDrawer
          mode={drawer.mode}
          alias={drawer.alias}
          domains={asList(domainsApi.data)}
          mailboxes={asList(mailboxesApi.data).map(m => m.username).filter(Boolean)}
          onClose={() => setDrawer(null)}
          onSaved={reload}
          onDelete={a => { setDrawer(null); setConfirmAlias(a); }}
        />
      )}
      {confirmAlias && (
        <ConfirmModal
          title={t('aliases.form.deleteTitle')}
          msg={t('aliases.form.deleteMsg', { alias: decodeIdnAddress(confirmAlias.address) })}
          cta={t('common.delete')}
          danger
          onCancel={() => setConfirmAlias(null)}
          onConfirm={doDelete}
        />
      )}
    </>
  );
}
