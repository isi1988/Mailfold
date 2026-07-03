import React from 'react';
import { ResourceManager } from '../components/ResourceManager.jsx';
import { useT } from '../i18n/index.jsx';

export function BccPage() {
  const t = useT();
  return (
    <ResourceManager
      endpoint="/api/bcc"
      idKey="id"
      title={t('advanced.bcc.title')}
      sub={t('advanced.bcc.sub')}
      addLabel={t('advanced.bcc.add')}
      filterKeys={['local_dest', 'bcc_dest']}
      filterPlaceholder={t('advanced.bcc.filter')}
      columns={[
        { key: 'local_dest', label: t('advanced.bcc.col.localDest'), w: '2fr', mono: true },
        { key: 'bcc_dest', label: t('advanced.bcc.col.bccDest'), w: '2fr', mono: true },
        { key: 'type', label: t('advanced.bcc.col.type'), w: '1fr',
          render: r => r.type === 'sender' ? t('advanced.bcc.type.sender') : t('advanced.bcc.type.rcpt') },
        { key: 'active', label: t('advanced.bcc.col.status'), w: '.8fr',
          render: r => (r.active === 1 || r.active === '1') ? t('common.active') : t('common.disabled') },
      ]}
      fields={[
        { key: 'local_dest', label: t('advanced.bcc.f.localDest'), type: 'text', mono: true,
          placeholder: 'user@example.com', hint: t('advanced.bcc.f.localDestHint') },
        { key: 'bcc_dest', label: t('advanced.bcc.f.bccDest'), type: 'text', mono: true,
          placeholder: 'archive@example.com' },
        { key: 'type', label: t('advanced.bcc.f.type'), type: 'select', default: 'rcpt',
          options: [
            { value: 'rcpt', label: t('advanced.bcc.type.rcpt') },
            { value: 'sender', label: t('advanced.bcc.type.sender') },
          ] },
        { key: 'active', label: t('common.active'), type: 'toggle', default: true },
      ]}
      canEdit={false}
      labels={{
        newTitle: t('advanced.bcc.newTitle'),
        empty: t('advanced.bcc.empty'),
        created: t('advanced.bcc.created'),
        deleted: t('advanced.bcc.deleted'),
        failed: t('advanced.bcc.failed'),
        deleteTitle: t('advanced.bcc.deleteTitle'),
        deleteMsg: (name) => t('advanced.bcc.deleteMsg', { name }),
      }}
      describe={r => r.bcc_dest || r.local_dest || String(r.id)}
    />
  );
}
