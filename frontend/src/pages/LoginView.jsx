import React, { useEffect, useState } from 'react';
import { Logo } from '../ds/components/atoms/Logo.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Label } from '../ds/components/atoms/Label.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { useAuth } from '../auth/AuthContext.jsx';
import { useWebmailAuth } from '../auth/WebmailAuthContext.jsx';
import { useDomainAdminAuth } from '../auth/DomainAdminAuthContext.jsx';
import { PasswordField } from '../components/PasswordField.jsx';
import { api } from '../api/client.js';
import { wm } from '../api/webmail.js';
import { domainAdminApi } from '../api/domainAdmin.js';
import { useT } from '../i18n/index.jsx';
import markUrl from '../ds/assets/mailfold-mark.png';

// The one login screen. It tries the admin, webmail (mailbox), and domain-admin
// logins with the same credentials in parallel: whichever succeed decide where
// the user goes, and if more than one does, the user is asked which to open.
export function LoginView() {
  const { applySession: applyAdmin } = useAuth();
  const { applySession: applyWebmail, verifyLogin2FA } = useWebmailAuth();
  const { applySession: applyDomainAdmin } = useDomainAdminAuth();
  const t = useT();
  const [user, setUser] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [choice, setChoice] = useState(null); // { admin, web, domainAdmin } — whichever succeeded, when more than one did
  const [pending, setPending] = useState(null); // { pendingToken, web, domainAdmin } while the admin's 2FA code is needed
  const [pendingWebmail, setPendingWebmail] = useState(null); // { pendingToken, mailbox } while only webmail needs a 2FA code
  const [code, setCode] = useState('');
  const [codeError, setCodeError] = useState('');
  const [webmailCode, setWebmailCode] = useState('');
  const [webmailCodeError, setWebmailCodeError] = useState('');
  const [screen, setScreen] = useState('signIn'); // 'signIn' | 'forgot' | 'device'
  const [forgotSent, setForgotSent] = useState(false);
  const [ssoProviders, setSsoProviders] = useState([]); // [{id, name}] for the domain part of whatever's typed so far
  const [ssoError, setSsoError] = useState('');
  const [deviceKey, setDeviceKey] = useState('');
  const [deviceError, setDeviceError] = useState('');

  // Resolve SSO providers for whatever domain the user has typed so far
  // (debounced) — there is no fixed global "is SSO on" flag any more: each
  // provider is scoped to specific domains (or every domain), so the set of
  // buttons to offer depends on what's typed into the email/username field.
  useEffect(() => {
    const at = user.lastIndexOf('@');
    const domain = at >= 0 ? user.slice(at + 1).trim() : '';
    if (!domain) {
      setSsoProviders([]);
      return;
    }
    let cancelled = false;
    const timer = setTimeout(() => {
      api.get('/api/auth/sso/providers?domain=' + encodeURIComponent(domain)).then(list => {
        if (!cancelled) setSsoProviders(Array.isArray(list) ? list : []);
      }).catch(() => { if (!cancelled) setSsoProviders([]); });
    }, 300);
    return () => { cancelled = true; clearTimeout(timer); };
  }, [user]);

  // The SSO callback redirects back here with the outcome in the URL fragment
  // (never the query string, so it never hits server logs or a Referer
  // header) — consume it once on mount, then scrub it from the address bar.
  // A successful SSO login always signs into a mailbox's webmail, never the
  // admin panel or a domain-admin session.
  useEffect(() => {
    const hash = window.location.hash;
    if (!hash || hash.length < 2) return;
    const params = new URLSearchParams(hash.slice(1));
    const token = params.get('sso_webmail_token');
    const email = params.get('sso_webmail_email');
    const err = params.get('sso_error');
    if (token && email) {
      applyWebmail(token, email);
    } else if (err) {
      setSsoError(err);
    }
    if (token || err) {
      window.history.replaceState(null, '', window.location.pathname + window.location.search);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function startSSO(providerId) {
    window.location.href = '/api/auth/sso/start?provider_id=' + encodeURIComponent(providerId);
  }

  async function submit(e) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError('');
    const id = user.trim();
    const [adminR, webR, domainAdminR] = await Promise.allSettled([
      api.post('/api/auth/login', { user: id, password }),
      wm.login(id, password),
      domainAdminApi.login(id, password),
    ]);
    const admin = adminR.status === 'fulfilled' ? adminR.value : null;
    const web = webR.status === 'fulfilled' ? webR.value : null;
    const domainAdmin = domainAdminR.status === 'fulfilled' ? domainAdminR.value : null;

    // A webmail login that itself needs a second factor carries no token yet;
    // only ride it along into the admin's own 2FA/choice screens when it's a
    // genuine success.
    const webOK = web && web.token ? web : null;
    const successes = [admin && { type: 'admin', value: admin }, domainAdmin && { type: 'domainAdmin', value: domainAdmin }, webOK && { type: 'web', value: webOK }].filter(Boolean);

    if (admin && admin.needs_2fa) {
      setPending({ pendingToken: admin.pending_token, web: webOK, domainAdmin });
      setBusy(false);
      return;
    }
    if (successes.length > 1) {
      setChoice({ admin, web: webOK, domainAdmin });
      setBusy(false);
      return;
    }
    if (admin) {
      applyAdmin(admin.token, admin.user);
      return;
    }
    if (domainAdmin) {
      applyDomainAdmin(domainAdmin.token, domainAdmin.user, domainAdmin.domains);
      return;
    }
    if (web && web.needs_2fa) {
      setPendingWebmail({ pendingToken: web.pending_token, mailbox: id });
      setBusy(false);
      return;
    }
    if (webOK) {
      applyWebmail(webOK.token, webOK.email || id);
      return;
    }
    setError(t('login.invalidCredentials'));
    setBusy(false);
  }

  async function submitWebmailCode(e) {
    e.preventDefault();
    if (busy || !webmailCode.trim()) return;
    setBusy(true);
    setWebmailCodeError('');
    try {
      await verifyLogin2FA(pendingWebmail.pendingToken, webmailCode.trim(), pendingWebmail.mailbox);
    } catch {
      setWebmailCodeError(t('login.twoFactor.invalidCode'));
      setBusy(false);
    }
  }

  async function submitCode(e) {
    e.preventDefault();
    if (busy || !code.trim()) return;
    setBusy(true);
    setCodeError('');
    try {
      const sess = await api.post('/api/auth/2fa/verify', { pending_token: pending.pendingToken, code: code.trim() });
      if (pending.web) applyWebmail(pending.web.token, pending.web.email || user.trim());
      if (pending.domainAdmin) applyDomainAdmin(pending.domainAdmin.token, pending.domainAdmin.user, pending.domainAdmin.domains);
      applyAdmin(sess.token, sess.user);
    } catch {
      setCodeError(t('login.twoFactor.invalidCode'));
      setBusy(false);
    }
  }

  function backToSignIn() {
    setPending(null);
    setCode('');
    setCodeError('');
    setPendingWebmail(null);
    setWebmailCode('');
    setWebmailCodeError('');
    setScreen('signIn');
    setForgotSent(false);
    setDeviceKey('');
    setDeviceError('');
  }

  // submitDevice signs into webmail with a personal API key instead of the
  // mailbox password — meant for a new device that only holds a key.
  async function submitDevice(e) {
    e.preventDefault();
    if (busy || !deviceKey.trim()) return;
    setBusy(true);
    setDeviceError('');
    try {
      const res = await wm.deviceLogin(deviceKey.trim());
      applyWebmail(res.token, res.email);
    } catch {
      setDeviceError(t('login.device.failed'));
      setBusy(false);
    }
  }

  async function submitForgot(e) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    try {
      await api.post('/api/auth/forgot-password', {});
    } catch {
      /* the endpoint always answers 200; a network error still shows the same
         generic confirmation so the flow can't be used to probe server state */
    } finally {
      setForgotSent(true);
      setBusy(false);
    }
  }

  // Every access level a successful login could have matched: opening the
  // admin panel keeps the webmail session too, so the in-panel Webmail page
  // works; opening webmail or the domain-admin panel commits only that one.
  function openAdmin() {
    if (choice.web) applyWebmail(choice.web.token, choice.web.email || user.trim());
    applyAdmin(choice.admin.token, choice.admin.user);
  }
  function openWebmail() {
    applyWebmail(choice.web.token, choice.web.email || user.trim());
  }
  function openDomainAdmin() {
    applyDomainAdmin(choice.domainAdmin.token, choice.domainAdmin.user, choice.domainAdmin.domains);
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
            {choice.admin && <Button variant="primary" block size="lg" style={{ marginTop: 26 }} onClick={openAdmin}>{t('login.choose.admin')}</Button>}
            {choice.domainAdmin && <Button variant="secondary" block size="lg" style={{ marginTop: 12 }} onClick={openDomainAdmin}>{t('login.choose.domainAdmin')}</Button>}
            {choice.web && <Button variant="secondary" block size="lg" style={{ marginTop: 12 }} onClick={openWebmail}>{t('login.choose.webmail')}</Button>}
          </div>
        ) : pending ? (
          <form className="mf-login__form" onSubmit={submitCode}>
            <div className="mf-login__title">{t('login.twoFactor.title')}</div>
            <div className="mf-login__sub">{t('login.twoFactor.sub')}</div>
            <div style={{ marginTop: 28 }}>
              <Label strong style={{ marginBottom: 7 }}>{t('login.twoFactor.codeLabel')}</Label>
              <Input
                size="lg"
                mono
                autoFocus
                placeholder={t('login.twoFactor.codePlaceholder')}
                autoComplete="one-time-code"
                value={code}
                onChange={e => setCode(e.target.value)}
              />
            </div>
            {codeError && <div className="mf-form-error" style={{ marginTop: 14 }} role="alert">{codeError}</div>}
            <Button variant="primary" block size="lg" type="submit" disabled={busy} style={{ marginTop: 22 }}>
              {busy ? t('login.twoFactor.verifying') : t('login.twoFactor.verify')}
            </Button>
            <Button variant="secondary" block size="lg" type="button" style={{ marginTop: 10 }} onClick={backToSignIn}>
              {t('login.twoFactor.back')}
            </Button>
          </form>
        ) : pendingWebmail ? (
          <form className="mf-login__form" onSubmit={submitWebmailCode}>
            <div className="mf-login__title">{t('login.twoFactor.title')}</div>
            <div className="mf-login__sub">{t('login.twoFactor.sub')}</div>
            <div style={{ marginTop: 28 }}>
              <Label strong style={{ marginBottom: 7 }}>{t('login.twoFactor.codeLabel')}</Label>
              <Input
                size="lg"
                mono
                autoFocus
                placeholder={t('login.twoFactor.codePlaceholder')}
                autoComplete="one-time-code"
                value={webmailCode}
                onChange={e => setWebmailCode(e.target.value)}
              />
            </div>
            {webmailCodeError && <div className="mf-form-error" style={{ marginTop: 14 }} role="alert">{webmailCodeError}</div>}
            <Button variant="primary" block size="lg" type="submit" disabled={busy} style={{ marginTop: 22 }}>
              {busy ? t('login.twoFactor.verifying') : t('login.twoFactor.verify')}
            </Button>
            <Button variant="secondary" block size="lg" type="button" style={{ marginTop: 10 }} onClick={backToSignIn}>
              {t('login.twoFactor.back')}
            </Button>
          </form>
        ) : screen === 'forgot' ? (
          <form className="mf-login__form" onSubmit={submitForgot}>
            <div className="mf-login__title">{t('login.forgot.title')}</div>
            <div className="mf-login__sub">{forgotSent ? t('login.forgot.sent') : t('login.forgot.sub')}</div>
            {!forgotSent && (
              <Button variant="primary" block size="lg" type="submit" disabled={busy} style={{ marginTop: 22 }}>
                {busy ? t('login.forgot.sending') : t('login.forgot.send')}
              </Button>
            )}
            <Button variant="secondary" block size="lg" type="button" style={{ marginTop: 12 }} onClick={backToSignIn}>
              {t('login.forgot.back')}
            </Button>
          </form>
        ) : screen === 'device' ? (
          <form className="mf-login__form" onSubmit={submitDevice}>
            <div className="mf-login__title">{t('login.device.title')}</div>
            <div className="mf-login__sub">{t('login.device.sub')}</div>
            <div style={{ marginTop: 28 }}>
              <Label strong style={{ marginBottom: 7 }}>{t('login.device.keyLabel')}</Label>
              <Input
                size="lg"
                mono
                autoFocus
                placeholder="mf_live_..."
                autoComplete="off"
                value={deviceKey}
                onChange={e => setDeviceKey(e.target.value)}
              />
            </div>
            {deviceError && <div className="mf-form-error" style={{ marginTop: 14 }} role="alert">{deviceError}</div>}
            <Button variant="primary" block size="lg" type="submit" disabled={busy} style={{ marginTop: 22 }}>
              {busy ? t('login.signingIn') : t('login.device.signIn')}
            </Button>
            <Button variant="secondary" block size="lg" type="button" style={{ marginTop: 10 }} onClick={backToSignIn}>
              {t('login.device.back')}
            </Button>
          </form>
        ) : (
          <form className="mf-login__form" onSubmit={submit}>
            <div className="mf-login__title">{t('login.signIn')}</div>
            <div className="mf-login__sub">{t('login.welcome')}</div>
            <div style={{ marginTop: 28 }}>
              <Label strong style={{ marginBottom: 7 }}>{t('login.emailOrUsername')}</Label>
              <Input size="lg" placeholder={t('login.usernamePlaceholder')} autoComplete="username" value={user} onChange={e => setUser(e.target.value)} />
            </div>
            <div style={{ marginTop: 16 }}>
              <div className="mf-row mf-row--between" style={{ marginBottom: 7 }}>
                <Label strong style={{ marginBottom: 0 }}>{t('login.password')}</Label>
                <span
                  role="button"
                  className="mf-u-faint"
                  style={{ fontSize: 12.5, cursor: 'pointer' }}
                  onClick={() => setScreen('forgot')}
                >
                  {t('login.forgotLink')}
                </span>
              </div>
              <PasswordField value={password} onChange={e => setPassword(e.target.value)} />
            </div>
            {(error || ssoError) && <div className="mf-form-error" style={{ marginTop: 14 }} role="alert">{error || t('login.sso.failed')}</div>}
            <Button variant="primary" block size="lg" type="submit" disabled={busy} style={{ marginTop: 22 }}>
              {busy ? t('login.signingIn') : t('login.signIn')}
            </Button>
            {ssoProviders.map(p => (
              <Button key={p.id} variant="secondary" block size="lg" type="button" style={{ marginTop: 12 }} onClick={() => startSSO(p.id)}>
                {t('login.sso.buttonNamed', { name: p.name })}
              </Button>
            ))}
            <div style={{ marginTop: 16, textAlign: 'center' }}>
              <span
                role="button"
                className="mf-u-faint"
                style={{ fontSize: 12.5, cursor: 'pointer' }}
                onClick={() => setScreen('device')}
              >
                {t('login.device.link')}
              </span>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
