import React from 'react';
import { ResourceManager } from '../components/ResourceManager.jsx';
import { useT } from '../i18n/index.jsx';

export function MailboxTemplatesPage() {
  const t = useT();
  return (
    <ResourceManager
      endpoint="/api/mailbox-templates"
      idKey="id"
      title={t('advanced.mailboxtemplates.title')}
      sub={t('advanced.mailboxtemplates.sub')}
      addLabel={t('advanced.mailboxtemplates.add')}
      filterKeys={['template']}
      filterPlaceholder={t('advanced.mailboxtemplates.filter')}
      columns={[
        { key: 'template', label: t('advanced.mailboxtemplates.col.name'), w: '2fr', mono: true },
        { key: 'quota', label: t('advanced.mailboxtemplates.col.quota'), w: '1fr',
          render: r => {
            const a = r.attributes || r;
            const q = a.quota;
            return (q == null || q === '') ? '—' : `${q} MB`;
          } },
        { key: 'active', label: t('advanced.mailboxtemplates.col.status'), w: '.8fr',
          render: r => {
            const a = r.attributes || r;
            return (a.active === 1 || a.active === '1') ? t('common.active') : t('common.disabled');
          } },
      ]}
      fields={[
        { key: 'template', label: t('advanced.mailboxtemplates.f.name'), type: 'text',
          placeholder: 'Standard mailbox', createOnly: true,
          hint: t('advanced.mailboxtemplates.f.nameHint') },
        { key: 'quota', label: t('advanced.mailboxtemplates.f.quota'), type: 'number', default: 3072,
          hint: t('advanced.mailboxtemplates.f.quotaHint') },
        { key: 'imap_access', label: t('advanced.mailboxtemplates.f.imap'), type: 'toggle', default: true },
        { key: 'pop3_access', label: t('advanced.mailboxtemplates.f.pop3'), type: 'toggle', default: true },
        { key: 'smtp_access', label: t('advanced.mailboxtemplates.f.smtp'), type: 'toggle', default: true },
        { key: 'sieve_access', label: t('advanced.mailboxtemplates.f.sieve'), type: 'toggle', default: true },
        { key: 'sogo_access', label: t('advanced.mailboxtemplates.f.sogo'), type: 'toggle', default: true },
        { key: 'tls_enforce_in', label: t('advanced.mailboxtemplates.f.tlsIn'), type: 'toggle', default: false },
        { key: 'tls_enforce_out', label: t('advanced.mailboxtemplates.f.tlsOut'), type: 'toggle', default: false },
        { key: 'force_pw_update', label: t('advanced.mailboxtemplates.f.forcePw'), type: 'toggle', default: false },
        { key: 'quarantine_notification', label: t('advanced.mailboxtemplates.f.quarantineNotif'), type: 'select',
          default: 'hourly',
          options: [
            { value: 'never', label: t('advanced.mailboxtemplates.opt.never') },
            { value: 'hourly', label: t('advanced.mailboxtemplates.opt.hourly') },
            { value: 'daily', label: t('advanced.mailboxtemplates.opt.daily') },
            { value: 'weekly', label: t('advanced.mailboxtemplates.opt.weekly') },
          ] },
        { key: 'active', label: t('common.active'), type: 'toggle', default: true },
      ]}
      canEdit={true}
      labels={{
        newTitle: t('advanced.mailboxtemplates.newTitle'), editTitle: t('advanced.mailboxtemplates.editTitle'),
        empty: t('advanced.mailboxtemplates.empty'),
        created: t('advanced.mailboxtemplates.created'), updated: t('advanced.mailboxtemplates.updated'),
        deleted: t('advanced.mailboxtemplates.deleted'), failed: t('advanced.mailboxtemplates.failed'),
        deleteTitle: t('advanced.mailboxtemplates.deleteTitle'),
        deleteMsg: (name) => t('advanced.mailboxtemplates.deleteMsg', { name }),
      }}
      describe={r => r.template || String(r.id)}
    />
  );
}
