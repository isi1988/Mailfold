import React from 'react';
import { ResourceManager } from '../components/ResourceManager.jsx';
import { decodeIdnAddress } from '../lib/idn.js';
import { useT } from '../i18n/index.jsx';

export function RecipientMapsPage() {
  const t = useT();
  return (
    <ResourceManager
      endpoint="/api/recipient-maps"
      idKey="id"
      title={t('advanced.recipientmaps.title')}
      sub={t('advanced.recipientmaps.sub')}
      addLabel={t('advanced.recipientmaps.add')}
      filterKeys={['recipient_map_old', 'recipient_map_new']}
      filterPlaceholder={t('advanced.recipientmaps.filter')}
      columns={[
        { key: 'recipient_map_old', label: t('advanced.recipientmaps.col.old'), w: '2fr', mono: true,
          render: r => decodeIdnAddress(r.recipient_map_old) },
        { key: 'recipient_map_new', label: t('advanced.recipientmaps.col.new'), w: '2fr', mono: true,
          render: r => decodeIdnAddress(r.recipient_map_new) },
        { key: 'active', label: t('advanced.recipientmaps.col.status'), w: '.8fr',
          render: r => (r.active === 1 || r.active === '1') ? t('common.active') : t('common.disabled') },
      ]}
      fields={[
        { key: 'recipient_map_old', label: t('advanced.recipientmaps.f.old'), type: 'text', mono: true, placeholder: 'old@example.com', hint: t('advanced.recipientmaps.f.oldHint') },
        { key: 'recipient_map_new', label: t('advanced.recipientmaps.f.new'), type: 'text', mono: true, placeholder: 'new@example.com', hint: t('advanced.recipientmaps.f.newHint') },
        { key: 'active', label: t('common.active'), type: 'toggle', default: true },
      ]}
      canEdit={false}
      labels={{
        newTitle: t('advanced.recipientmaps.newTitle'),
        empty: t('advanced.recipientmaps.empty'),
        created: t('advanced.recipientmaps.created'),
        deleted: t('advanced.recipientmaps.deleted'),
        failed: t('advanced.recipientmaps.failed'),
        deleteTitle: t('advanced.recipientmaps.deleteTitle'),
        deleteMsg: (name) => t('advanced.recipientmaps.deleteMsg', { name }),
      }}
      describe={r => decodeIdnAddress(r.recipient_map_old || String(r.id))}
    />
  );
}
