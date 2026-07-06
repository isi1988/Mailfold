import React, { useState } from 'react';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { Pagination } from '../ds/components/molecules/Pagination.jsx';
import { Pill } from '../ds/components/atoms/Pill.jsx';
import { Table, TableRow } from '../ds/components/organisms/Table.jsx';
import { AsyncView } from '../components/States.jsx';
import { useApi } from '../lib/useApi.js';
import { useT } from '../i18n/index.jsx';

// actionLabel turns a raw action string ("login", "login_failed", "logout",
// or "METHOD /path" for a generic mutating action) into something readable,
// without needing a translation entry per possible admin route.
function actionLabel(t, action) {
  if (action === 'login') return t('auditlog.action.login');
  if (action === 'login_failed') return t('auditlog.action.loginFailed');
  if (action === 'logout') return t('auditlog.action.logout');
  return action;
}

function fmtWhen(iso) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}

export function AuditLogPage() {
  const t = useT();
  const [page, setPage] = useState(1);
  const { data, loading, error, reload } = useApi('/api/audit-log?page=' + page, [page]);

  const entries = (data && data.entries) || [];
  const total = (data && data.total) || 0;
  const pageSize = (data && data.page_size) || 50;
  const pageCount = Math.max(1, Math.ceil(total / pageSize));
  const from = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const to = Math.min(page * pageSize, total);

  return (
    <>
      <PageHeader title={t('auditlog.title')} sub={t('auditlog.sub')} />
      <AsyncView loading={loading} error={error} reload={reload} empty={entries.length === 0 ? t('auditlog.empty') : null}>
        <Table columns={[
          { label: t('auditlog.col.when'), w: '1.4fr' },
          { label: t('auditlog.col.actor'), w: '1.2fr' },
          { label: t('auditlog.col.action'), w: '1.6fr' },
          { label: t('auditlog.col.status'), w: '.7fr' },
          { label: t('auditlog.col.ip'), w: '1fr' },
        ]}>
          {entries.map(e => (
            <TableRow key={e.id}>
              <span className="mf-u-faint mf-u-mono" style={{ fontSize: 12.5 }}>{fmtWhen(e.at)}</span>
              <span className="mf-row" style={{ gap: 6 }}>
                <span className="mf-truncate" style={{ fontSize: 13 }}>{e.actor || '—'}</span>
                <Pill tone={e.actor_type === 'admin' ? 'blue' : 'neutral'}>
                  {e.actor_type === 'admin' ? t('auditlog.actorType.admin') : t('auditlog.actorType.domainAdmin')}
                </Pill>
              </span>
              <span className="mf-u-mono mf-truncate" style={{ fontSize: 12.5 }}>{actionLabel(t, e.action)}</span>
              <span className={e.status >= 400 ? 'mf-u-red' : 'mf-u-faint'} style={{ fontSize: 12.5 }}>{e.status}</span>
              <span className="mf-u-faint mf-u-mono" style={{ fontSize: 12 }}>{e.ip}</span>
            </TableRow>
          ))}
        </Table>
        {total > 0 && (
          <Pagination page={page} pageCount={pageCount} summary={t('common.showing', { from, to, total })} onPage={setPage} style={{ marginTop: 14 }} />
        )}
      </AsyncView>
    </>
  );
}
