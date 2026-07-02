import React, { useState } from 'react';
import { Logo } from '../ds/components/atoms/Logo.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Label } from '../ds/components/atoms/Label.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { useAuth } from '../auth/AuthContext.jsx';
import { useT } from '../i18n/index.jsx';
import markUrl from '../ds/assets/mailfold-mark.png';

// Sign-in screen — the only view without the app chrome. Wires the design's
// Login markup to the /api/auth/login flow.
export function LoginView() {
  const { login } = useAuth();
  const t = useT();
  const [user, setUser] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit(e) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError('');
    try {
      await login(user.trim(), password);
    } catch (err) {
      setError(err && err.status === 401 ? t('login.invalidCredentials') : (err.message || t('login.failed')));
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
          <div className="mf-login__headline">{t('login.headline')}</div>
          <p className="mf-login__lede">{t('login.lede')}</p>
          <div className="mf-login__badge">{t('login.badgePrefix')} <span className="mf-u-strong mf-u-mono">mailcow</span></div>
        </div>
        <div className="mf-login__foot">{t('login.foot')}</div>
      </div>

      <div className="mf-login__panel">
        <form className="mf-login__form" onSubmit={submit}>
          <div className="mf-login__title">{t('login.signIn')}</div>
          <div className="mf-login__sub">{t('login.welcome')}</div>
          <div style={{ marginTop: 28 }}>
            <Label strong style={{ marginBottom: 7 }}>{t('login.emailOrUsername')}</Label>
            <Input
              size="lg"
              placeholder={t('login.usernamePlaceholder')}
              autoComplete="username"
              value={user}
              onChange={e => setUser(e.target.value)}
            />
          </div>
          <div style={{ marginTop: 16 }}>
            <div className="mf-row" style={{ marginBottom: 7 }}>
              <Label strong style={{ marginBottom: 0 }}>{t('login.password')}</Label>
            </div>
            <Input
              size="lg"
              type="password"
              placeholder="••••••••••"
              autoComplete="current-password"
              value={password}
              onChange={e => setPassword(e.target.value)}
            />
          </div>
          {error && (
            <div className="mf-u-danger" style={{ marginTop: 14, fontSize: 13 }} role="alert">{error}</div>
          )}
          <Button variant="primary" block size="lg" type="submit" disabled={busy} style={{ marginTop: 22 }}>
            {busy ? t('login.signingIn') : t('login.signIn')}
          </Button>
        </form>
      </div>
    </div>
  );
}
