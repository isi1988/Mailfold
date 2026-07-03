import React from 'react';
import { ResourceManager } from '../components/ResourceManager.jsx';
import { useT } from '../i18n/index.jsx';

export function RelayHostsPage() {
  const t = useT();
  return (
    <ResourceManager
      endpoint="/api/relayhosts"
      idKey="id"
      title={t('advanced.relayhosts.title')}
      sub={t('advanced.relayhosts.sub')}
      addLabel={t('advanced.relayhosts.add')}
      filterKeys={['hostname', 'username']}
      filterPlaceholder={t('advanced.relayhosts.filter')}
      columns={[
        { key: 'hostname', label: t('advanced.relayhosts.col.hostname'), w: '2fr', mono: true },
        { key: 'username', label: t('advanced.relayhosts.col.username'), w: '1.5fr' },
        {
          key: 'active', label: t('advanced.relayhosts.col.status'), w: '.8fr',
          render: r => (r.active === 1 || r.active === '1') ? t('common.active') : t('common.disabled'),
        },
      ]}
      fields={[
        { key: 'hostname', label: t('advanced.relayhosts.f.hostname'), type: 'text', mono: true, placeholder: 'smtp.example.com:587', hint: t('advanced.relayhosts.f.hostnameHint') },
        { key: 'username', label: t('advanced.relayhosts.f.username'), type: 'text' },
        { key: 'password', label: t('advanced.relayhosts.f.password'), type: 'text', mono: true },
        { key: 'active', label: t('common.active'), type: 'toggle', default: true },
      ]}
      canEdit={true}
      labels={{
        newTitle: t('advanced.relayhosts.newTitle'), editTitle: t('advanced.relayhosts.editTitle'),
        empty: t('advanced.relayhosts.empty'),
        created: t('advanced.relayhosts.created'), updated: t('advanced.relayhosts.updated'),
        deleted: t('advanced.relayhosts.deleted'), failed: t('advanced.relayhosts.failed'),
        deleteTitle: t('advanced.relayhosts.deleteTitle'),
        deleteMsg: (name) => t('advanced.relayhosts.deleteMsg', { name }),
      }}
      describe={r => r.hostname || String(r.id)}
    />
  );
}
