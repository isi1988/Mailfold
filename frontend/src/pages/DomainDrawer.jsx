import React, { useState } from 'react';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Toggle } from '../ds/components/atoms/Toggle.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Logo } from '../ds/components/atoms/Logo.jsx';
import { api } from '../api/client.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';
import { isActive } from '../lib/format.js';

// The list exposes the per-mailbox max quota in bytes; the write API takes MB.
const bytesToGB = b => Math.max(1, Math.round((Number(b) || 0) / 1024 / 1024 / 1024)) || 1;

function errText(err, fallback) {
  return (err && err.body && err.body.message) || (err && err.message) || fallback;
}

export function DomainDrawer({ mode, domain, onClose, onSaved, onDelete }) {
  const t = useT();
  const { toast } = useToast();
  const editing = mode === 'edit';

  const [name, setName] = useState('');
  const [description, setDescription] = useState(editing ? domain.description || '' : '');
  const [quotaGB, setQuotaGB] = useState(editing ? bytesToGB(domain.max_quota_for_domain) : 10);
  const [maxMailboxes, setMaxMailboxes] = useState(10);
  const [active, setActive] = useState(editing ? isActive(domain.active) : true);
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (busy) return;
    if (!editing && !/^[^\s/@]+\.[^\s/@]+$/.test(name.trim())) {
      toast(t('domains.form.invalid'));
      return;
    }
    setBusy(true);
    const quotaMB = quotaGB * 1024;
    try {
      if (editing) {
        await api.put('/api/domains', {
          items: [domain.domain_name],
          attr: { description, maxquota: String(quotaMB), quota: String(quotaMB), active: active ? '1' : '0' },
        });
        toast(t('domains.form.updated', { domain: domain.domain_name }));
      } else {
        await api.post('/api/domains', {
          domain: name.trim(),
          description,
          aliases: '400',
          mailboxes: String(maxMailboxes),
          defquota: String(Math.min(3072, quotaMB)),
          maxquota: String(quotaMB),
          quota: String(quotaMB),
          active: active ? '1' : '0',
        });
        toast(t('domains.form.created', { domain: name.trim() }));
      }
      onSaved();
      onClose();
    } catch (err) {
      toast(t('domains.form.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  const footer = (
    <>
      {editing && onDelete && <Button variant="danger" onClick={() => onDelete(domain)}>{t('common.delete')}</Button>}
      <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
      <Button variant="primary" onClick={submit} disabled={busy}>
        {busy ? t('domains.form.saving') : (editing ? t('common.save') : t('domains.form.create'))}
      </Button>
    </>
  );

  return (
    <Drawer
      title={editing ? t('domains.form.editTitle') : t('domains.form.newTitle')}
      subtitle={editing ? domain.domain_name : name}
      icon={<div className="mf-avatar mf-avatar--square mf-avatar--34"><Logo wordmark={false} markSize={18} color="var(--accent-ink)" /></div>}
      footer={footer}
      onClose={onClose}
    >
      {!editing && (
        <FormField label={t('domains.form.domain')}>
          <Input mono placeholder="example.com" value={name} onChange={e => setName(e.target.value)} />
        </FormField>
      )}
      <FormField label={t('domains.form.description')}>
        <Input placeholder="Acme Inc." value={description} onChange={e => setDescription(e.target.value)} />
      </FormField>
      <FormField label={t('domains.form.quota')}>
        <Input type="number" min="1" align="right" value={quotaGB} onChange={e => setQuotaGB(Number(e.target.value) || 1)} />
      </FormField>
      {!editing && (
        <FormField label={t('domains.form.maxMailboxes')}>
          <Input type="number" min="1" align="right" value={maxMailboxes} onChange={e => setMaxMailboxes(Number(e.target.value) || 1)} />
        </FormField>
      )}
      <div className="mf-row mf-row--between" style={{ marginTop: 6 }}>
        <span className="mf-u-muted" style={{ fontSize: 13 }}>{t('domains.form.active')}</span>
        <Toggle on={active} onClick={() => setActive(a => !a)} style={{ cursor: 'pointer' }} />
      </div>
    </Drawer>
  );
}
