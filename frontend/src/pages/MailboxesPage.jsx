import React, { useState } from 'react';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { FilterTabs } from '../ds/components/molecules/FilterTabs.jsx';
import { SearchInput } from '../ds/components/molecules/SearchInput.jsx';
import { Table, TableRow } from '../ds/components/organisms/Table.jsx';
import { Pagination } from '../ds/components/molecules/Pagination.jsx';
import { Avatar } from '../ds/components/atoms/Avatar.jsx';
import { Pill } from '../ds/components/atoms/Pill.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { ConfirmModal } from '../ds/components/organisms/ConfirmModal.jsx';
import { initials, tone } from '../ds/data/sample.js';
import { useApi } from '../lib/useApi.js';
import { api } from '../api/client.js';
import { AsyncView } from '../components/States.jsx';
import { useToast } from '../components/Toast.jsx';
import { isActive, humanKB, asList } from '../lib/format.js';
import { useT } from '../i18n/index.jsx';
import { MailboxDrawer } from './MailboxDrawer.jsx';

// Filter tabs keyed by a stable value with a translated label.
const TABS = [
  { value: 'All', key: 'all' },
  { value: 'Active', key: 'active' },
  { value: 'Disabled', key: 'disabled' },
];

const PAGE_SIZE = 20;

export function MailboxesPage() {
  const t = useT();
  const { toast } = useToast();
  const { data, loading, error, reload } = useApi('/api/mailboxes', []);
  const domainsApi = useApi('/api/domains', []);
  const [tab, setTab] = useState('All');
  const [q, setQ] = useState('');
  const [page, setPage] = useState(1);
  const [drawer, setDrawer] = useState(null); // { mode:'create' } | { mode:'edit', mailbox }
  const [confirmMb, setConfirmMb] = useState(null);

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
    { label: t('mailboxes.col.mailbox'), w: '2.3fr' },
    { label: t('mailboxes.col.domain'), w: '1fr' },
    { label: t('mailboxes.col.quota'), w: '1.1fr' },
    { label: t('mailboxes.col.messages'), w: '1fr' },
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
        actions={<Button variant="primary" onClick={() => setDrawer({ mode: 'create' })}>{t('mailboxes.new')}</Button>}
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
              <span className="mf-u-faint mf-u-mono" style={{ fontSize: 12.5 }}>{humanKB(m.quota)}</span>
              <span className="mf-u-faint mf-u-mono" style={{ fontSize: 12.5 }}>{m.messages ?? 0}</span>
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
    </>
  );
}
