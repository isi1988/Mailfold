import React from 'react';
import { ResourceManager } from '../components/ResourceManager.jsx';
import { decodeIdnDomain } from '../lib/idn.js';
import { useT } from '../i18n/index.jsx';

export function ForwardingHostsPage() {
  const t = useT();
  return (
    <ResourceManager
      endpoint="/api/forwarding-hosts"
      idKey="host"
      title={t('advanced.fwdhosts.title')}
      sub={t('advanced.fwdhosts.sub')}
      addLabel={t('advanced.fwdhosts.add')}
      filterKeys={['host', 'source']}
      filterPlaceholder={t('advanced.fwdhosts.filter')}
      columns={[
        { key: 'host', label: t('advanced.fwdhosts.col.host'), w: '2fr', mono: true,
          render: r => decodeIdnDomain(r.host) },
        { key: 'source', label: t('advanced.fwdhosts.col.source'), w: '2fr', mono: true,
          render: r => decodeIdnDomain(r.source) },
        { key: 'keep_spam', label: t('advanced.fwdhosts.col.keepSpam'), w: '1fr',
          render: r => (r.keep_spam === 'yes' || r.keep_spam === 1 || r.keep_spam === '1')
            ? t('advanced.fwdhosts.keepSpamOn')
            : t('advanced.fwdhosts.keepSpamOff') },
      ]}
      fields={[
        { key: 'hostname', label: t('advanced.fwdhosts.f.hostname'), type: 'text', mono: true,
          placeholder: 'smtp.example.com', hint: t('advanced.fwdhosts.f.hostnameHint') },
        { key: 'keep_spam', label: t('advanced.fwdhosts.f.keepSpam'), type: 'select',
          default: 'no',
          options: [
            { value: 'no', label: t('advanced.fwdhosts.keepSpamOff') },
            { value: 'yes', label: t('advanced.fwdhosts.keepSpamOn') },
          ],
          hint: t('advanced.fwdhosts.f.keepSpamHint') },
      ]}
      canEdit={false}
      labels={{
        newTitle: t('advanced.fwdhosts.newTitle'),
        empty: t('advanced.fwdhosts.empty'),
        created: t('advanced.fwdhosts.created'),
        deleted: t('advanced.fwdhosts.deleted'),
        failed: t('advanced.fwdhosts.failed'),
        deleteTitle: t('advanced.fwdhosts.deleteTitle'),
        deleteMsg: (name) => t('advanced.fwdhosts.deleteMsg', { name }),
      }}
      describe={r => decodeIdnDomain(r.host || r.source || '')}
    />
  );
}
