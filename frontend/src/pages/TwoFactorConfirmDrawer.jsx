import React, { useState } from 'react';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { PasswordField } from '../components/PasswordField.jsx';
import { RecoveryCodesBox } from './RecoveryCodesBox.jsx';
import { useT } from '../i18n/index.jsx';

/**
 * Shared shape for the two 2FA actions that only need the current password:
 * disabling two-factor auth, and regenerating recovery codes.
 *   mode      'disable' | 'regenerate'
 *   onConfirm (password) => Promise<result> — for 'regenerate', result.recovery_codes
 *             is revealed once in this same drawer; throw to keep it open with an error
 *   onClose   () => void
 */
export function TwoFactorConfirmDrawer({ mode, onConfirm, onClose }) {
  const t = useT();
  const prefix = 'settings.security.twoFactor.' + (mode === 'disable' ? 'disableModal' : 'regenerateModal');
  const copy = {
    title: t(prefix + '.title'),
    desc: mode === 'regenerate' ? t(prefix + '.desc') : '',
    currentPassword: t(prefix + '.currentPassword'),
    confirm: t(prefix + '.confirm'),
    confirming: t(prefix + '.confirming'),
  };
  const e = k => t('settings.security.twoFactor.enroll.' + k);
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [revealedCodes, setRevealedCodes] = useState(null);

  async function submit() {
    if (busy || !password) return;
    setBusy(true);
    setError('');
    try {
      const result = await onConfirm(password);
      if (mode === 'regenerate') {
        setRevealedCodes((result && result.recovery_codes) || []);
      } else {
        onClose();
      }
    } catch (err) {
      setError((err && err.message) || t('settings.security.twoFactor.failed'));
    } finally {
      setBusy(false);
    }
  }

  if (revealedCodes) {
    const footer = <Button variant="primary" className="mf-spacer" onClick={onClose}>{e('done')}</Button>;
    return (
      <Drawer title={e('titleCodes')} footer={footer} onClose={onClose}>
        <div className="mf-u-faint" style={{ fontSize: 12.5, marginBottom: 14, lineHeight: 1.5 }}>{e('codesIntro')}</div>
        <RecoveryCodesBox codes={revealedCodes} copyLabel={e('copy')} />
      </Drawer>
    );
  }

  const footer = (
    <>
      <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
      <Button variant={mode === 'disable' ? 'danger' : 'primary'} onClick={submit} disabled={busy}>
        {busy ? copy.confirming : copy.confirm}
      </Button>
    </>
  );
  return (
    <Drawer title={copy.title} footer={footer} onClose={onClose}>
      {copy.desc && <div className="mf-u-faint" style={{ fontSize: 12.5, marginBottom: 14 }}>{copy.desc}</div>}
      <FormField label={copy.currentPassword}>
        <PasswordField autoComplete="current-password" value={password} onChange={e2 => setPassword(e2.target.value)} />
      </FormField>
      {error && <div className="mf-form-error" role="alert">{error}</div>}
    </Drawer>
  );
}
