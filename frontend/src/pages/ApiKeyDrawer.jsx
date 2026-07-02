import React, { useState } from 'react';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { api } from '../api/client.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';

const SCOPES = [
  { id: 'mail:send', key: 'send' },
  { id: 'mail:read', key: 'read' },
  { id: 'mail:write', key: 'write' },
];

function errText(err, fallback) {
  return (err && err.body && err.body.message) || (err && err.message) || fallback;
}

/**
 * Mint an API key in a right-hand slide-over. After a successful mint the drawer
 * switches to a one-time reveal of the full token (it is never retrievable
 * again), then the caller refetches on close.
 *   onClose  () => void
 *   onSaved  () => void
 */
export function ApiKeyDrawer({ onClose, onSaved }) {
  const t = useT();
  const { toast } = useToast();
  const [mailbox, setMailbox] = useState('');
  const [label, setLabel] = useState('');
  const [scopes, setScopes] = useState({ 'mail:send': true, 'mail:read': true, 'mail:write': false });
  const [expiryDays, setExpiryDays] = useState(0);
  const [busy, setBusy] = useState(false);
  const [created, setCreated] = useState(null); // the mint response (holds the one-time token)

  const chosen = SCOPES.filter(s => scopes[s.id]).map(s => s.id);

  async function submit() {
    if (busy) return;
    if (!mailbox.trim() || chosen.length === 0) {
      toast(t('apikeys.form.invalid'));
      return;
    }
    setBusy(true);
    try {
      const body = { mailbox: mailbox.trim(), label: label.trim(), scopes: chosen };
      if (Number(expiryDays) > 0) body.expires_in_seconds = Math.round(Number(expiryDays) * 86400);
      const res = await api.post('/api/apikeys', body);
      setCreated(res);
      onSaved();
    } catch (err) {
      toast(t('apikeys.form.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  async function copyToken() {
    try {
      await navigator.clipboard.writeText(created.token);
      toast(t('apikeys.form.copied'));
    } catch {
      /* clipboard may be unavailable; the field is selectable as a fallback */
    }
  }

  // ---- one-time token reveal ----
  if (created) {
    const footer = <Button variant="primary" className="mf-spacer" onClick={onClose}>{t('apikeys.form.done')}</Button>;
    return (
      <Drawer title={t('apikeys.form.createdTitle')} subtitle={created.mailbox} footer={footer} onClose={onClose}>
        <div className="mf-u-danger" style={{ fontSize: 13, marginBottom: 12 }}>{t('apikeys.form.onceWarning')}</div>
        <FormField label={t('apikeys.form.token')}>
          <Input type="text" mono readonly value={created.token} onFocus={e => e.target.select()} />
        </FormField>
        <Button variant="secondary" size="sm" onClick={copyToken}>{t('apikeys.form.copy')}</Button>
        <div className="mf-u-faint" style={{ fontSize: 12, marginTop: 16 }}>{t('apikeys.form.usageHint')}</div>
      </Drawer>
    );
  }

  // ---- mint form ----
  const footer = (
    <>
      <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
      <Button variant="primary" onClick={submit} disabled={busy}>{busy ? t('apikeys.form.creating') : t('apikeys.form.create')}</Button>
    </>
  );
  return (
    <Drawer title={t('apikeys.form.newTitle')} subtitle={t('apikeys.sub')} footer={footer} onClose={onClose}>
      <FormField label={t('apikeys.form.mailbox')}>
        <Input placeholder="you@example.com" autoComplete="off" value={mailbox} onChange={e => setMailbox(e.target.value)} />
      </FormField>
      <FormField label={t('apikeys.form.label')}>
        <Input placeholder={t('apikeys.form.labelPlaceholder')} value={label} onChange={e => setLabel(e.target.value)} />
      </FormField>
      <FormField label={t('apikeys.form.scopes')}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {SCOPES.map(s => (
            <label key={s.id} className="mf-row mf-row--between" style={{ cursor: 'pointer', gap: 10 }}>
              <span>
                <span style={{ fontSize: 13.5, color: 'var(--ink)' }}>{t('apikeys.scope.' + s.key)}</span>
                <span className="mf-u-faint" style={{ fontSize: 12, marginLeft: 8 }}>{t('apikeys.scopeHint.' + s.key)}</span>
              </span>
              <input
                type="checkbox"
                checked={!!scopes[s.id]}
                onChange={e => setScopes(v => ({ ...v, [s.id]: e.target.checked }))}
              />
            </label>
          ))}
        </div>
      </FormField>
      <FormField label={t('apikeys.form.expiry')}>
        <Input type="number" min="0" align="right" value={expiryDays} onChange={e => setExpiryDays(Number(e.target.value) || 0)} />
      </FormField>
      <div className="mf-u-faint" style={{ fontSize: 12, marginTop: 2 }}>{t('apikeys.form.expiryHint')}</div>
    </Drawer>
  );
}
