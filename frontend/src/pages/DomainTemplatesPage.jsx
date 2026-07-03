import React from 'react';
import { ResourceManager } from '../components/ResourceManager.jsx';
import { useT } from '../i18n/index.jsx';

export function DomainTemplatesPage() {
  const t = useT();
  return (
    <ResourceManager
      endpoint="/api/domain-templates"
      idKey="id"
      title={t('advanced.domaintemplates.title')}
      sub={t('advanced.domaintemplates.sub')}
      addLabel={t('advanced.domaintemplates.add')}
      filterKeys={['template']}
      filterPlaceholder={t('advanced.domaintemplates.filter')}
      columns={[
        { key: 'template', label: t('advanced.domaintemplates.col.name'), w: '2fr', mono: true },
        { key: 'max_num_mboxes_for_domain', label: t('advanced.domaintemplates.col.maxMailboxes'), w: '1fr',
          render: r => (r.attributes && r.attributes.max_num_mboxes_for_domain) ?? r.max_num_mboxes_for_domain ?? '—' },
        { key: 'max_quota_for_domain', label: t('advanced.domaintemplates.col.maxQuota'), w: '1fr',
          render: r => (r.attributes && r.attributes.max_quota_for_domain) ?? r.max_quota_for_domain ?? '—' },
        { key: 'active', label: t('advanced.domaintemplates.col.status'), w: '.8fr',
          render: r => {
            const v = (r.attributes && r.attributes.active) ?? r.active;
            return (v === 1 || v === '1') ? t('common.active') : t('common.disabled');
          } },
      ]}
      fields={[
        { key: 'template', label: t('advanced.domaintemplates.f.name'), type: 'text',
          placeholder: 'standard', createOnly: true,
          hint: t('advanced.domaintemplates.f.nameHint') },
        { key: 'max_num_mboxes_for_domain', label: t('advanced.domaintemplates.f.maxMailboxes'), type: 'number', default: 10 },
        { key: 'max_num_aliases_for_domain', label: t('advanced.domaintemplates.f.maxAliases'), type: 'number', default: 400 },
        { key: 'max_quota_for_domain', label: t('advanced.domaintemplates.f.maxQuota'), type: 'number', default: 10240,
          hint: t('advanced.domaintemplates.f.quotaHint') },
        { key: 'def_quota_for_mbox', label: t('advanced.domaintemplates.f.defQuota'), type: 'number', default: 3072,
          hint: t('advanced.domaintemplates.f.quotaHint') },
        { key: 'gal', label: t('advanced.domaintemplates.f.gal'), type: 'toggle', default: true,
          hint: t('advanced.domaintemplates.f.galHint') },
        { key: 'backupmx', label: t('advanced.domaintemplates.f.backupmx'), type: 'toggle', default: false },
        { key: 'relay_all_recipients', label: t('advanced.domaintemplates.f.relayAll'), type: 'toggle', default: false },
        { key: 'active', label: t('common.active'), type: 'toggle', default: true },
      ]}
      canEdit={true}
      labels={{
        newTitle: t('advanced.domaintemplates.newTitle'),
        editTitle: t('advanced.domaintemplates.editTitle'),
        empty: t('advanced.domaintemplates.empty'),
        created: t('advanced.domaintemplates.created'),
        updated: t('advanced.domaintemplates.updated'),
        deleted: t('advanced.domaintemplates.deleted'),
        failed: t('advanced.domaintemplates.failed'),
        deleteTitle: t('advanced.domaintemplates.deleteTitle'),
        deleteMsg: (name) => t('advanced.domaintemplates.deleteMsg', { name }),
      }}
      describe={r => r.template || String(r.id)}
    />
  );
}
