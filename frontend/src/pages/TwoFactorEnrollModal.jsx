import React, { useState } from 'react';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { PasswordField } from '../components/PasswordField.jsx';
import { RecoveryCodesBox } from './RecoveryCodesBox.jsx';
import { api } from '../api/client.js';
import { useT } from '../i18n/index.jsx';

/**
 * Three-step two-factor enrollment wizard: confirm the current password,
 * scan the QR code and enter a code to prove it works, then save the
 * one-time recovery codes.
 *   onClose  () => void
 *   onSaved  () => void — called once 2FA is actually enabled
 */
export function TwoFactorEnrollModal({ onClose, onSaved }) {
  const t = useT();
  const e = k => t('settings.security.twoFactor.enroll.' + k);
  const [step, setStep] = useState('confirm'); // confirm | scan | codes
  const [password, setPassword] = useState('');
  const [enrollment, setEnrollment] = useState(null); // { secret, otpauth_uri, qr_data_uri }
  const [code, setCode] = useState('');
  const [recoveryCodes, setRecoveryCodes] = useState([]);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function confirmPassword() {
    if (busy || !password) return;
    setBusy(true);
    setError('');
    try {
      const res = await api.post('/api/account/2fa/enroll', { current_password: password });
      setEnrollment(res);
      setStep('scan');
    } catch (err) {
      setError((err && err.message) || t('settings.security.twoFactor.failed'));
    } finally {
      setBusy(false);
    }
  }

  async function confirmCode() {
    if (busy || !code.trim()) return;
    setBusy(true);
    setError('');
    try {
      const res = await api.post('/api/account/2fa/confirm', { code: code.trim() });
      setRecoveryCodes(res.recovery_codes || []);
      setStep('codes');
    } catch (err) {
      setError(e('invalidCode'));
    } finally {
      setBusy(false);
    }
  }

  function finish() {
    onSaved();
    onClose();
  }

  if (step === 'confirm') {
    const footer = (
      <>
        <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
        <Button variant="primary" onClick={confirmPassword} disabled={busy}>
          {busy ? t('settings.security.password.saving') : e('continue')}
        </Button>
      </>
    );
    return (
      <Drawer title={e('titleConfirm')} footer={footer} onClose={onClose}>
        <FormField label={e('currentPassword')}>
          <PasswordField autoComplete="current-password" value={password} onChange={ev => setPassword(ev.target.value)} />
        </FormField>
        {error && <div className="mf-form-error" role="alert">{error}</div>}
      </Drawer>
    );
  }

  if (step === 'scan') {
    const footer = (
      <>
        <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
        <Button variant="primary" onClick={confirmCode} disabled={busy}>
          {busy ? e('confirming') : e('confirm')}
        </Button>
      </>
    );
    return (
      <Drawer title={e('titleScan')} footer={footer} onClose={onClose}>
        <div className="mf-u-faint" style={{ fontSize: 12.5, marginBottom: 14, lineHeight: 1.5 }}>{e('scanHint')}</div>
        {enrollment && enrollment.qr_data_uri && (
          <div style={{ display: 'flex', justifyContent: 'center', marginBottom: 14 }}>
            <img src={enrollment.qr_data_uri} alt="" width={180} height={180} style={{ borderRadius: 10, border: '1px solid var(--hair)' }} />
          </div>
        )}
        <div style={{ fontSize: 12, marginBottom: 14 }}>
          <div className="mf-u-faint" style={{ marginBottom: 5 }}>{e('manualEntry')}</div>
          <div className="mf-u-mono" style={{ wordBreak: 'break-all', fontSize: 12.5 }}>{enrollment && enrollment.secret}</div>
        </div>
        <FormField label={e('code')}>
          <Input mono placeholder="123456" autoComplete="one-time-code" value={code} onChange={ev => setCode(ev.target.value)} />
        </FormField>
        {error && <div className="mf-form-error" role="alert">{error}</div>}
      </Drawer>
    );
  }

  // step === 'codes'
  const footer = <Button variant="primary" className="mf-spacer" onClick={finish}>{e('done')}</Button>;
  return (
    <Drawer title={e('titleCodes')} footer={footer} onClose={finish}>
      <div className="mf-u-faint" style={{ fontSize: 12.5, marginBottom: 14, lineHeight: 1.5 }}>{e('codesIntro')}</div>
      <RecoveryCodesBox codes={recoveryCodes} copyLabel={e('copy')} />
    </Drawer>
  );
}
