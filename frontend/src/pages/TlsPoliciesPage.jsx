import React from 'react';
import { ResourceManager } from '../components/ResourceManager.jsx';
import { useT } from '../i18n/index.jsx';

export function TlsPoliciesPage() {
  const t = useT();
  return (
    <ResourceManager
      endpoint="/api/tls-policies"
      idKey="id"
      title={t('advanced.tlspolicies.title')}
      sub={t('advanced.tlspolicies.sub')}
      addLabel={t('advanced.tlspolicies.add')}
      filterKeys={['dest', 'policy']}
      filterPlaceholder={t('advanced.tlspolicies.filter')}
      columns={[
        { key: 'dest', label: t('advanced.tlspolicies.col.dest'), w: '2fr', mono: true },
        { key: 'policy', label: t('advanced.tlspolicies.col.policy'), w: '1.2fr' },
        { key: 'parameters', label: t('advanced.tlspolicies.col.parameters'), w: '1.5fr', mono: true,
          render: r => r.parameters || '—' },
        { key: 'active', label: t('advanced.tlspolicies.col.status'), w: '.8fr',
          render: r => (r.active === 1 || r.active === '1') ? t('common.active') : t('common.disabled') },
      ]}
      fields={[
        { key: 'dest', label: t('advanced.tlspolicies.f.dest'), type: 'text', mono: true,
          placeholder: 'example.com', hint: t('advanced.tlspolicies.f.destHint') },
        { key: 'policy', label: t('advanced.tlspolicies.f.policy'), type: 'select', default: 'encrypt',
          options: [
            { value: 'none', label: t('advanced.tlspolicies.policy.none') },
            { value: 'may', label: t('advanced.tlspolicies.policy.may') },
            { value: 'encrypt', label: t('advanced.tlspolicies.policy.encrypt') },
            { value: 'dane', label: t('advanced.tlspolicies.policy.dane') },
            { value: 'dane-only', label: t('advanced.tlspolicies.policy.daneOnly') },
            { value: 'fingerprint', label: t('advanced.tlspolicies.policy.fingerprint') },
            { value: 'verify', label: t('advanced.tlspolicies.policy.verify') },
            { value: 'secure', label: t('advanced.tlspolicies.policy.secure') },
          ] },
        { key: 'parameters', label: t('advanced.tlspolicies.f.parameters'), type: 'text', mono: true,
          placeholder: 'match=.example.com', hint: t('advanced.tlspolicies.f.parametersHint') },
        { key: 'active', label: t('common.active'), type: 'toggle', default: true },
      ]}
      canEdit={false}
      labels={{
        newTitle: t('advanced.tlspolicies.newTitle'),
        empty: t('advanced.tlspolicies.empty'),
        created: t('advanced.tlspolicies.created'),
        deleted: t('advanced.tlspolicies.deleted'),
        failed: t('advanced.tlspolicies.failed'),
        deleteTitle: t('advanced.tlspolicies.deleteTitle'),
        deleteMsg: (name) => t('advanced.tlspolicies.deleteMsg', { name }),
      }}
      describe={r => r.dest || String(r.id)}
    />
  );
}
