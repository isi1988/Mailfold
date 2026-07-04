import React, { useState } from 'react';
import { Logo } from '../ds/components/atoms/Logo.jsx';
import { Label } from '../ds/components/atoms/Label.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { PasswordField } from '../components/PasswordField.jsx';
import { api } from '../api/client.js';
import { useT } from '../i18n/index.jsx';
import markUrl from '../ds/assets/mailfold-mark.png';

// Reads the reset token from the URL's query string. This page is reached
// straight from an emailed link, before any session exists, so it never uses
// the router — just the raw location.
function tokenFromLocation() {
  return new URLSearchParams(window.location.search).get('token') || '';
}

// Public page for /reset?token=... — the second half of the forgot-password
// flow. It never requires (or shows) authentication.
export function ResetPasswordPage() {
  const t = useT();
  const token = tokenFromLocation();
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);

  async function submit(e) {
    e.preventDefault();
    if (busy) return;
    setError('');
    if (newPassword.length < 8) {
      setError(t('resetPassword.tooShort'));
      return;
    }
    if (newPassword !== confirmPassword) {
      setError(t('resetPassword.mismatch'));
      return;
    }
    setBusy(true);
    try {
      await api.post('/api/auth/reset-password', { token, new_password: newPassword });
      setDone(true);
    } catch (err) {
      setError((err && err.message) || t('resetPassword.failed'));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mf-login">
      <div className="mf-login__hero">
        <div className="mf-login__watermark">
          <svg width="460" height="460" viewBox="0 0 26 26" fill="none">
            <rect x="3.4" y="3.4" width="19.2" height="19.2" rx="5.5" stroke="var(--ink)" strokeWidth="1" />
            <path d="M15.4 3.5 H22.5 V10.6 Z" fill="var(--ink)" />
          </svg>
        </div>
        <Logo size="md" style={{ position: 'relative' }} />
        <div className="mf-login__center">
          <img src={markUrl} alt="Mailfold" style={{ width: 'min(480px,100%)', height: 'auto', display: 'block' }} />
        </div>
        <div className="mf-login__foot">{t('login.foot')}</div>
      </div>

      <div className="mf-login__panel">
        {!token ? (
          <div className="mf-login__form">
            <div className="mf-login__title">{t('resetPassword.title')}</div>
            <div className="mf-form-error" style={{ marginTop: 14 }} role="alert">{t('resetPassword.missingToken')}</div>
            <Button variant="secondary" block size="lg" style={{ marginTop: 22 }} onClick={() => { window.location.href = '/'; }}>
              {t('resetPassword.backToLogin')}
            </Button>
          </div>
        ) : done ? (
          <div className="mf-login__form">
            <div className="mf-login__title">{t('resetPassword.title')}</div>
            <div className="mf-login__sub">{t('resetPassword.success')}</div>
            <Button variant="primary" block size="lg" style={{ marginTop: 22 }} onClick={() => { window.location.href = '/'; }}>
              {t('resetPassword.backToLogin')}
            </Button>
          </div>
        ) : (
          <form className="mf-login__form" onSubmit={submit}>
            <div className="mf-login__title">{t('resetPassword.title')}</div>
            <div className="mf-login__sub">{t('resetPassword.sub')}</div>
            <div style={{ marginTop: 28 }}>
              <Label strong style={{ marginBottom: 7 }}>{t('resetPassword.newPassword')}</Label>
              <PasswordField autoComplete="new-password" value={newPassword} onChange={e => setNewPassword(e.target.value)} />
            </div>
            <div style={{ marginTop: 16 }}>
              <Label strong style={{ marginBottom: 7 }}>{t('resetPassword.confirmPassword')}</Label>
              <PasswordField autoComplete="new-password" value={confirmPassword} onChange={e => setConfirmPassword(e.target.value)} />
            </div>
            {error && <div className="mf-form-error" style={{ marginTop: 14 }} role="alert">{error}</div>}
            <Button variant="primary" block size="lg" type="submit" disabled={busy} style={{ marginTop: 22 }}>
              {busy ? t('resetPassword.submitting') : t('resetPassword.submit')}
            </Button>
          </form>
        )}
      </div>
    </div>
  );
}
