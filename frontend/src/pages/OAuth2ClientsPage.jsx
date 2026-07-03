import React from 'react';
import { ResourceManager } from '../components/ResourceManager.jsx';
import { useT } from '../i18n/index.jsx';

export function OAuth2ClientsPage() {
  const t = useT();
  return (
    <ResourceManager
      endpoint="/api/oauth2-clients"
      idKey="id"
      title={t('advanced.oauth2.title')}
      sub={t('advanced.oauth2.sub')}
      addLabel={t('advanced.oauth2.add')}
      filterKeys={['client_id', 'redirect_uri']}
      filterPlaceholder={t('advanced.oauth2.filter')}
      columns={[
        { key: 'client_id', label: t('advanced.oauth2.col.clientId'), w: '1.5fr', mono: true },
        { key: 'redirect_uri', label: t('advanced.oauth2.col.redirectUri'), w: '2fr', mono: true },
        { key: 'scope', label: t('advanced.oauth2.col.scope'), w: '1fr', mono: true,
          render: r => r.scope || '—' },
      ]}
      fields={[
        { key: 'redirect_uri', label: t('advanced.oauth2.f.redirectUri'), type: 'text', mono: true,
          placeholder: 'https://app.example.com/callback',
          hint: t('advanced.oauth2.f.redirectUriHint') },
        { key: 'scope', label: t('advanced.oauth2.f.scope'), type: 'text', mono: true,
          default: 'profile', hint: t('advanced.oauth2.f.scopeHint') },
      ]}
      canEdit={false}
      labels={{
        newTitle: t('advanced.oauth2.newTitle'),
        empty: t('advanced.oauth2.empty'),
        created: t('advanced.oauth2.created'),
        deleted: t('advanced.oauth2.deleted'),
        failed: t('advanced.oauth2.failed'),
        deleteTitle: t('advanced.oauth2.deleteTitle'),
        deleteMsg: (name) => t('advanced.oauth2.deleteMsg', { name }),
      }}
      describe={r => r.client_id || String(r.id)}
    />
  );
}
