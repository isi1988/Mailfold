import React, { useState } from 'react';
import { Logo } from '../ds/components/atoms/Logo.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Label } from '../ds/components/atoms/Label.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { useAuth } from '../auth/AuthContext.jsx';
import { useWebmailAuth } from '../auth/WebmailAuthContext.jsx';
import { PasswordField } from '../components/PasswordField.jsx';
import { api } from '../api/client.js';
import { wm } from '../api/webmail.js';
import { useT } from '../i18n/index.jsx';
import markUrl from '../ds/assets/mailfold-mark.png';

// The one login screen. It tries the admin and the webmail (mailbox) logins with
// the same credentials in parallel: whichever succeeds decides where the user
// goes, and when both succeed the user is asked which to open.
export function LoginView() {
  const { applySession: applyAdmin } = useAuth();
  const { applySession: applyWebmail } = useWebmailAuth();
  const t = useT();
  const [user, setUser] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [choice, setChoice] = useState(null); // { admin, web } when both succeed

  async function submit(e) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError('');
    const id = user.trim();
    const [adminR, webR] = await Promise.allSettled([
      api.post('/api/auth/login', { user: id, password }),
      wm.login(id, password),
    ]);
    const admin = adminR.status === 'fulfilled' ? adminR.value : null;
    const web = webR.status === 'fulfilled' ? webR.value : null;

    if (admin && web) {
      setChoice({ admin, web });
      setBusy(false);
      return;
    }
    if (admin) {
      applyAdmin(admin.token, admin.user);
      return;
    }
    if (web) {
      applyWebmail(web.token, web.email || id);
      return;
    }
    setError(t('login.invalidCredentials'));
    setBusy(false);
  }

  // Both access levels: opening the admin panel keeps the webmail session too, so
  // the in-panel Webmail page works; opening webmail commits only that session.
  function openAdmin() {
    applyWebmail(choice.web.token, choice.web.email || user.trim());
    applyAdmin(choice.admin.token, choice.admin.user);
  }
  function openWebmail() {
    applyWebmail(choice.web.token, choice.web.email || user.trim());
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
        {choice ? (
          <div className="mf-login__form">
            <div className="mf-login__title">{t('login.choose.title')}</div>
            <div className="mf-login__sub">{t('login.choose.sub')}</div>
            <Button variant="primary" block size="lg" style={{ marginTop: 26 }} onClick={openAdmin}>{t('login.choose.admin')}</Button>
            <Button variant="secondary" block size="lg" style={{ marginTop: 12 }} onClick={openWebmail}>{t('login.choose.webmail')}</Button>
          </div>
        ) : (
          <form className="mf-login__form" onSubmit={submit}>
            <div className="mf-login__title">{t('login.signIn')}</div>
            <div className="mf-login__sub">{t('login.welcome')}</div>
            <div style={{ marginTop: 28 }}>
              <Label strong style={{ marginBottom: 7 }}>{t('login.emailOrUsername')}</Label>
              <Input size="lg" placeholder={t('login.usernamePlaceholder')} autoComplete="username" value={user} onChange={e => setUser(e.target.value)} />
            </div>
            <div style={{ marginTop: 16 }}>
              <div className="mf-row" style={{ marginBottom: 7 }}>
                <Label strong style={{ marginBottom: 0 }}>{t('login.password')}</Label>
              </div>
              <PasswordField value={password} onChange={e => setPassword(e.target.value)} />
            </div>
            {error && <div className="mf-u-danger" style={{ marginTop: 14, fontSize: 13 }} role="alert">{error}</div>}
            <Button variant="primary" block size="lg" type="submit" disabled={busy} style={{ marginTop: 22 }}>
              {busy ? t('login.signingIn') : t('login.signIn')}
            </Button>
          </form>
        )}
      </div>
    </div>
  );
}
