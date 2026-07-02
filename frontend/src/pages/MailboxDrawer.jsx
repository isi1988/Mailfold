import React, { useState } from 'react';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Toggle } from '../ds/components/atoms/Toggle.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Avatar } from '../ds/components/atoms/Avatar.jsx';
import { initials } from '../ds/data/sample.js';
import { api } from '../api/client.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';
import { isActive } from '../lib/format.js';

// mailcow quota is megabytes on write; the list returns kilobytes.
const kbToGB = kb => Math.max(1, Math.round((Number(kb) || 0) / 1024 / 1024)) || 1;

function randomPassword() {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789!@#$%';
  let out = '';
  const buf = new Uint32Array(16);
  crypto.getRandomValues(buf);
  for (let i = 0; i < buf.length; i += 1) out += chars[buf[i] % chars.length];
  return out;
}

function errText(err, fallback) {
  return (err && err.body && err.body.message) || (err && err.message) || fallback;
}

/**
 * Create / edit a mailbox in a right-hand slide-over.
 *   mode      'create' | 'edit'
 *   mailbox   the row (edit mode)
 *   domains   [{domain_name}]  for the create form's domain picker
 *   onClose   () => void
 *   onSaved   () => void   — parent refetches the list
 */
export function MailboxDrawer({ mode, mailbox, domains = [], onClose, onSaved, onDelete }) {
  const t = useT();
  const { toast } = useToast();
  const editing = mode === 'edit';

  const [localPart, setLocalPart] = useState('');
  const [domain, setDomain] = useState(domains[0] ? domains[0].domain_name : '');
  const [name, setName] = useState(editing ? mailbox.name || '' : '');
  const [quotaGB, setQuotaGB] = useState(editing ? kbToGB(mailbox.quota) : 3);
  const [password, setPassword] = useState('');
  const [active, setActive] = useState(editing ? isActive(mailbox.active) : true);
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (busy) return;
    if (!editing && (!localPart.trim() || !domain || password.length < 8)) {
      toast(t('mailboxes.form.invalid'));
      return;
    }
    setBusy(true);
    try {
      if (editing) {
        const attr = { name, quota: quotaGB * 1024, active: active ? '1' : '0' };
        if (password) {
          attr.password = password;
          attr.password2 = password;
        }
        await api.put('/api/mailboxes', { items: [mailbox.username], attr });
        toast(t('mailboxes.form.updated', { mailbox: mailbox.username }));
      } else {
        await api.post('/api/mailboxes', {
          local_part: localPart.trim(),
          domain,
          name,
          quota: quotaGB * 1024,
          password,
          password2: password,
          active: active ? '1' : '0',
        });
        toast(t('mailboxes.form.created', { mailbox: localPart.trim() + '@' + domain }));
      }
      onSaved();
      onClose();
    } catch (err) {
      toast(t('mailboxes.form.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  const addr = editing ? mailbox.username : (localPart ? localPart + '@' + domain : '');
  const footer = (
    <>
      {editing && onDelete && (
        <Button variant="danger" onClick={() => onDelete(mailbox)}>{t('common.delete')}</Button>
      )}
      <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
      <Button variant="primary" onClick={submit} disabled={busy}>
        {busy ? t('mailboxes.form.saving') : (editing ? t('common.save') : t('mailboxes.form.create'))}
      </Button>
    </>
  );

  return (
    <Drawer
      title={editing ? t('mailboxes.form.editTitle') : t('mailboxes.form.newTitle')}
      subtitle={addr}
      icon={<Avatar size={38}>{initials(name || addr || '?')}</Avatar>}
      footer={footer}
      onClose={onClose}
    >
      {!editing && (
        <div className="mf-row" style={{ gap: 10 }}>
          <FormField label={t('mailboxes.form.localPart')} style={{ flex: 1 }}>
            <Input placeholder="jane" value={localPart} onChange={e => setLocalPart(e.target.value)} />
          </FormField>
          <FormField label={t('mailboxes.form.domain')} style={{ flex: 1 }}>
            <select className="mf-input" value={domain} onChange={e => setDomain(e.target.value)}>
              {domains.map(d => <option key={d.domain_name} value={d.domain_name}>{d.domain_name}</option>)}
            </select>
          </FormField>
        </div>
      )}

      <FormField label={t('mailboxes.form.name')}>
        <Input placeholder="Jane Doe" value={name} onChange={e => setName(e.target.value)} />
      </FormField>

      <FormField label={t('mailboxes.form.quota')}>
        <Input type="number" min="1" align="right" value={quotaGB} onChange={e => setQuotaGB(Number(e.target.value) || 1)} />
      </FormField>

      <FormField label={editing ? t('mailboxes.form.resetPassword') : t('mailboxes.form.password')}>
        <div className="mf-row" style={{ gap: 8 }}>
          <Input
            className="mf-spacer"
            type="text"
            mono
            placeholder={editing ? t('mailboxes.form.leaveBlank') : '••••••••'}
            value={password}
            onChange={e => setPassword(e.target.value)}
          />
          <Button variant="secondary" size="sm" onClick={() => setPassword(randomPassword())}>{t('mailboxes.form.generate')}</Button>
        </div>
      </FormField>

      <div className="mf-row mf-row--between" style={{ marginTop: 6 }}>
        <span className="mf-u-muted" style={{ fontSize: 13 }}>{t('mailboxes.form.active')}</span>
        <Toggle on={active} onClick={() => setActive(a => !a)} style={{ cursor: 'pointer' }} />
      </div>
    </Drawer>
  );
}
