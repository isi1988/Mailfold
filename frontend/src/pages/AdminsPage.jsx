import React from 'react';
import { ResourceManager } from '../components/ResourceManager.jsx';
import { useT } from '../i18n/index.jsx';

export function AdminsPage() {
  const t = useT();
  return (
    <ResourceManager
      endpoint="/api/admins"
      idKey="username"
      title={t('advanced.admins.title')}
      sub={t('advanced.admins.sub')}
      addLabel={t('advanced.admins.add')}
      filterKeys={['username']}
      filterPlaceholder={t('advanced.admins.filter')}
      columns={[
        { key: 'username', label: t('advanced.admins.col.username'), w: '2fr', mono: true },
        { key: 'active', label: t('advanced.admins.col.status'), w: '.8fr',
          render: r => (r.active === 1 || r.active === '1') ? t('common.active') : t('common.disabled') },
      ]}
      fields={[
        { key: 'username', label: t('advanced.admins.f.username'), type: 'text', createOnly: true, placeholder: 'admin' },
        { key: 'password', label: t('advanced.admins.f.password'), type: 'text', mono: true, hint: t('advanced.admins.f.passwordHint') },
        { key: 'password2', label: t('advanced.admins.f.password2'), type: 'text', mono: true },
        { key: 'active', label: t('common.active'), type: 'toggle', default: true },
      ]}
      canEdit={true}
      labels={{
        newTitle: t('advanced.admins.newTitle'), editTitle: t('advanced.admins.editTitle'),
        empty: t('advanced.admins.empty'),
        created: t('advanced.admins.created'), updated: t('advanced.admins.updated'),
        deleted: t('advanced.admins.deleted'), failed: t('advanced.admins.failed'),
        deleteTitle: t('advanced.admins.deleteTitle'),
        deleteMsg: (name) => t('advanced.admins.deleteMsg', { name }),
      }}
      describe={r => r.username || ''}
    />
  );
}
