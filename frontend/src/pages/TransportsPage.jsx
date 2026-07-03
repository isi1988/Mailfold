import React from 'react';
import { ResourceManager } from '../components/ResourceManager.jsx';
import { useT } from '../i18n/index.jsx';

export function TransportsPage() {
  const t = useT();
  return (
    <ResourceManager
      endpoint="/api/transports"
      idKey="id"
      title={t('advanced.transports.title')}
      sub={t('advanced.transports.sub')}
      addLabel={t('advanced.transports.add')}
      filterKeys={['destination', 'nexthop', 'username']}
      filterPlaceholder={t('advanced.transports.filter')}
      columns={[
        { key: 'destination', label: t('advanced.transports.col.destination'), w: '2fr', mono: true },
        { key: 'nexthop', label: t('advanced.transports.col.nexthop'), w: '2fr', mono: true },
        { key: 'username', label: t('advanced.transports.col.username'), w: '1.5fr' },
        { key: 'active', label: t('advanced.transports.col.status'), w: '.8fr',
          render: r => (r.active === 1 || r.active === '1') ? t('common.active') : t('common.disabled') },
      ]}
      fields={[
        { key: 'destination', label: t('advanced.transports.f.destination'), type: 'text', mono: true, placeholder: 'example.com', hint: t('advanced.transports.f.destinationHint') },
        { key: 'nexthop', label: t('advanced.transports.f.nexthop'), type: 'text', mono: true, placeholder: '[smtp.example.com]:587', hint: t('advanced.transports.f.nexthopHint') },
        { key: 'username', label: t('advanced.transports.f.username'), type: 'text' },
        { key: 'password', label: t('advanced.transports.f.password'), type: 'text', mono: true },
        { key: 'active', label: t('common.active'), type: 'toggle', default: true },
      ]}
      canEdit={true}
      labels={{
        newTitle: t('advanced.transports.newTitle'), editTitle: t('advanced.transports.editTitle'),
        empty: t('advanced.transports.empty'),
        created: t('advanced.transports.created'), updated: t('advanced.transports.updated'),
        deleted: t('advanced.transports.deleted'), failed: t('advanced.transports.failed'),
        deleteTitle: t('advanced.transports.deleteTitle'),
        deleteMsg: (name) => t('advanced.transports.deleteMsg', { name }),
      }}
      describe={r => r.destination || String(r.id)}
    />
  );
}
