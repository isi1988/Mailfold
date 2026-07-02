import React, { useState } from 'react';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { SearchInput } from '../ds/components/molecules/SearchInput.jsx';
import { Table, TableRow } from '../ds/components/organisms/Table.jsx';
import { Pagination } from '../ds/components/molecules/Pagination.jsx';
import { Logo } from '../ds/components/atoms/Logo.jsx';
import { Pill } from '../ds/components/atoms/Pill.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { ProgressBar } from '../ds/components/atoms/ProgressBar.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { tone } from '../ds/data/sample.js';
import { useApi } from '../lib/useApi.js';
import { AsyncView } from '../components/States.jsx';
import { isActive, human, asList } from '../lib/format.js';
import { useT } from '../i18n/index.jsx';

const PAGE_SIZE = 20;

function storagePct(used, max) {
  if (!max || max <= 0) return 0;
  return Math.min(100, Math.round((used / max) * 100));
}

export function DomainsPage() {
  const t = useT();
  const { data, loading, error, reload } = useApi('/api/domains', []);
  const [q, setQ] = useState('');
  const [page, setPage] = useState(1);

  const cols = [
    { label: t('domains.col.domain'), w: '2fr' },
    { label: t('domains.col.mailboxes'), w: '.8fr' },
    { label: t('domains.col.aliases'), w: '.8fr' },
    { label: t('domains.col.storage'), w: '1.5fr' },
    { label: t('domains.col.status'), w: '.9fr' },
    { label: '', w: '18px' },
  ];

  const rows = asList(data);
  const filtered = q
    ? rows.filter(d => (d.domain_name || '').toLowerCase().includes(q.toLowerCase()))
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
        title={t('domains.title')}
        sub={t('domains.count', { count: rows.length })}
        actions={<Button variant="primary">{t('domains.add')}</Button>}
      />
      <div className="mf-row" style={{ marginBottom: 14 }}>
        <SearchInput className="mf-spacer" style={{ width: 250 }} placeholder={t('domains.filter')} value={q} onChange={onQuery} />
      </div>

      <AsyncView
        loading={loading}
        error={error}
        reload={reload}
        empty={filtered.length === 0 ? (rows.length ? t('domains.emptyFilter') : t('domains.empty')) : null}
      >
        <Table columns={cols}>
          {paged.map(d => {
            const used = Number(d.bytes_total) || 0;
            const max = Number(d.max_quota_for_domain) || 0;
            return (
              <TableRow key={d.domain_name}>
                <div className="mf-cell-user">
                  <div className="mf-avatar mf-avatar--square mf-avatar--34">
                    <Logo wordmark={false} markSize={18} color="var(--accent-ink)" />
                  </div>
                  <div className="mf-min0">
                    <div className="mf-u-mono" style={{ fontSize: 14, fontWeight: 600, color: 'var(--ink)' }}>{d.domain_name}</div>
                    {d.description ? <div className="mf-u-faint mf-truncate" style={{ fontSize: 11.5, marginTop: 2 }}>{d.description}</div> : null}
                  </div>
                </div>
                <span style={{ fontSize: 13.5, color: 'var(--ink)', fontWeight: 500 }}>{d.mboxes_in_domain ?? 0}</span>
                <span className="mf-u-muted" style={{ fontSize: 13 }}>{d.aliases_in_domain ?? 0}</span>
                <div>
                  <div className="mf-u-muted" style={{ fontSize: 11, marginBottom: 5 }}>
                    {human(used)}{max > 0 ? ' / ' + human(max) : ''}
                  </div>
                  <ProgressBar pct={storagePct(used, max)} />
                </div>
                <span><Pill tone={isActive(d.active) ? tone('active') : 'neutral'}>{isActive(d.active) ? t('common.active') : t('common.disabled')}</Pill></span>
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
    </>
  );
}
