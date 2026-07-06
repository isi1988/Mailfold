import React, { useState } from 'react';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { PasswordField } from '../components/PasswordField.jsx';
import { api } from '../api/client.js';
import { createPasskey } from '../lib/webauthn.js';
import { useT } from '../i18n/index.jsx';

/**
 * Enrolls a new passkey/security key: confirm the current password (the same
 * gate two-factor enrollment uses, so a hijacked bearer token can't silently
 * plant a persistent backdoor credential), name the device, then hand off to
 * the browser's native WebAuthn prompt.
 *   onClose  () => void
 *   onSaved  () => void — called once the new credential is actually stored
 */
export function PasskeyEnrollDrawer({ onClose, onSaved }) {
  const t = useT();
  const e = k => t('settings.security.passkeys.add.' + k);
  const [password, setPassword] = useState('');
  const [name, setName] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (busy || !password) return;
    setBusy(true);
    setError('');
    try {
      const options = await api.post('/api/account/webauthn/register/begin', { current_password: password });
      const credential = await createPasskey(options);
      await api.post('/api/account/webauthn/register/finish?name=' + encodeURIComponent(name.trim()), credential);
      onSaved();
      onClose();
    } catch (err) {
      if (err && err.name === 'NotAllowedError') {
        setError(e('cancelled'));
      } else {
        setError((err && err.message) || e('failed'));
      }
    } finally {
      setBusy(false);
    }
  }

  const footer = (
    <>
      <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
      <Button variant="primary" onClick={submit} disabled={busy}>
        {busy ? e('adding') : e('continue')}
      </Button>
    </>
  );
  return (
    <Drawer title={e('title')} footer={footer} onClose={onClose}>
      <div className="mf-u-faint" style={{ fontSize: 12.5, marginBottom: 14, lineHeight: 1.5 }}>{e('hint')}</div>
      <FormField label={e('currentPassword')}>
        <PasswordField autoComplete="current-password" value={password} onChange={ev => setPassword(ev.target.value)} />
      </FormField>
      <FormField label={e('nameLabel')}>
        <Input placeholder={e('namePlaceholder')} value={name} onChange={ev => setName(ev.target.value)} />
      </FormField>
      {error && <div className="mf-form-error" role="alert">{error}</div>}
    </Drawer>
  );
}
