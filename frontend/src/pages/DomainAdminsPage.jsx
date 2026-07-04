import React from 'react';
import { ResourceManager } from '../components/ResourceManager.jsx';
import { useApi } from '../lib/useApi.js';
import { asList } from '../lib/format.js';
import { decodeIdnAddress, decodeIdnDomain } from '../lib/idn.js';
import { useT } from '../i18n/index.jsx';

export function DomainAdminsPage() {
  const t = useT();
  const domainsApi = useApi('/api/domains', []);
  const domainNames = asList(domainsApi.data).map(d => d.domain_name);
  return (
    <ResourceManager
      endpoint="/api/domain-admins"
      idKey="username"
      title={t('advanced.domainadmins.title')}
      sub={t('advanced.domainadmins.sub')}
      addLabel={t('advanced.domainadmins.add')}
      filterKeys={['username', 'selected_domains']}
      filterPlaceholder={t('advanced.domainadmins.filter')}
      columns={[
        { key: 'username', label: t('advanced.domainadmins.col.username'), w: '1.5fr', mono: true,
          render: r => decodeIdnAddress(r.username) },
        { key: 'selected_domains', label: t('advanced.domainadmins.col.domains'), w: '2fr',
          render: r => {
            const d = r.selected_domains;
            if (Array.isArray(d)) return d.length ? d.map(decodeIdnDomain).join(', ') : '—';
            return d ? decodeIdnDomain(d) : '—';
          } },
        { key: 'active', label: t('advanced.domainadmins.col.status'), w: '.8fr',
          render: r => (r.active === 1 || r.active === '1') ? t('common.active') : t('common.disabled') },
      ]}
      fields={[
        { key: 'username', label: t('advanced.domainadmins.f.username'), type: 'text', mono: true, createOnly: true },
        { key: 'password', label: t('advanced.domainadmins.f.password'), type: 'text', mono: true, hint: t('advanced.domainadmins.f.passwordHint') },
        { key: 'password2', label: t('advanced.domainadmins.f.password2'), type: 'text', mono: true },
        { key: 'domains', label: t('advanced.domainadmins.f.domains'), type: 'domain-multiselect',
          readKey: 'selected_domains', options: domainNames, decorate: decodeIdnDomain,
          emptyLabel: t('advanced.domainadmins.f.domainsEmpty'), hint: t('advanced.domainadmins.f.domainsHint') },
        { key: 'active', label: t('common.active'), type: 'toggle', default: true },
      ]}
      canEdit={true}
      labels={{
        newTitle: t('advanced.domainadmins.newTitle'), editTitle: t('advanced.domainadmins.editTitle'),
        empty: t('advanced.domainadmins.empty'),
        created: t('advanced.domainadmins.created'), updated: t('advanced.domainadmins.updated'),
        deleted: t('advanced.domainadmins.deleted'), failed: t('advanced.domainadmins.failed'),
        deleteTitle: t('advanced.domainadmins.deleteTitle'),
        deleteMsg: (name) => t('advanced.domainadmins.deleteMsg', { name }),
      }}
      describe={r => decodeIdnAddress(r.username || '')}
    />
  );
}
