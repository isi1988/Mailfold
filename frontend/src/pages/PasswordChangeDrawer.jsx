import React, { useState } from 'react';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { PasswordField } from '../components/PasswordField.jsx';
import { api } from '../api/client.js';
import { useT } from '../i18n/index.jsx';

/**
 * Change the admin password. Requires the current password so a hijacked
 * session token alone cannot silently take over the account.
 *   onClose  () => void
 *   onSaved  () => void
 */
export function PasswordChangeDrawer({ onClose, onSaved }) {
  const t = useT();
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [confirm, setConfirm] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (busy) return;
    setError('');
    if (next.length < 8) {
      setError(t('settings.security.password.tooShort'));
      return;
    }
    if (next !== confirm) {
      setError(t('settings.security.password.mismatch'));
      return;
    }
    setBusy(true);
    try {
      await api.post('/api/account/password', { current_password: current, new_password: next });
      onSaved();
    } catch (err) {
      setError((err && err.message) || t('settings.security.password.failed'));
    } finally {
      setBusy(false);
    }
  }

  const footer = (
    <>
      <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
      <Button variant="primary" onClick={submit} disabled={busy}>
        {busy ? t('settings.security.password.saving') : t('settings.security.password.save')}
      </Button>
    </>
  );
  return (
    <Drawer title={t('settings.security.password.title')} footer={footer} onClose={onClose}>
      <FormField label={t('settings.security.password.current')}>
        <PasswordField autoComplete="current-password" value={current} onChange={e => setCurrent(e.target.value)} />
      </FormField>
      <FormField label={t('settings.security.password.new')}>
        <PasswordField autoComplete="new-password" value={next} onChange={e => setNext(e.target.value)} />
      </FormField>
      <FormField label={t('settings.security.password.confirm')}>
        <PasswordField autoComplete="new-password" value={confirm} onChange={e => setConfirm(e.target.value)} />
      </FormField>
      {error && <div className="mf-form-error" role="alert">{error}</div>}
    </Drawer>
  );
}
