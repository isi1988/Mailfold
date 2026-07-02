import React from 'react';
import { Logo } from '../components/atoms/Logo.jsx';
import { Input } from '../components/atoms/Input.jsx';
import { Label } from '../components/atoms/Label.jsx';
import { Button } from '../components/atoms/Button.jsx';

/** Sign-in screen — the only page without the app chrome. */
export function Login() {
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
          <img src="./assets/mailfold-mark.png" alt="Mailfold" style={{ width: 'min(480px,100%)', height: 'auto', display: 'block' }} />
          <div className="mf-login__headline">A calm shell for your mail server.</div>
          <p className="mf-login__lede">Domains and mailboxes are your herd. Mailfold is the fold that keeps them — every letter, every gate, in one quiet place.</p>
          <div className="mf-login__badge">A modern admin UI for <span className="mf-u-strong mf-u-mono">mailcow</span></div>
        </div>
        <div className="mf-login__foot">self-hosted · powered by mailcow · v2025.07</div>
      </div>

      <div className="mf-login__panel">
        <div className="mf-login__form">
          <div className="mf-login__title">Sign in</div>
          <div className="mf-login__sub">Welcome back — enter your admin credentials.</div>
          <div style={{ marginTop: 28 }}>
            <Label strong style={{ marginBottom: 7 }}>Email</Label>
            <Input size="lg" placeholder="you@example.com" />
          </div>
          <div style={{ marginTop: 16 }}>
            <div className="mf-row" style={{ marginBottom: 7 }}>
              <Label strong style={{ marginBottom: 0 }}>Password</Label>
              <a className="mf-u-accent mf-spacer" style={{ fontSize: 12, cursor: 'pointer' }}>Forgot?</a>
            </div>
            <Input size="lg" type="password" placeholder="••••••••••" />
          </div>
          <Button variant="primary" block size="lg" style={{ marginTop: 22 }}>Sign in</Button>
          <div className="mf-or"><span>or</span></div>
          <Button variant="secondary" block size="lg">Single sign-on (SSO)</Button>
          <div style={{ marginTop: 24, textAlign: 'center', fontSize: 12, color: 'var(--faint)' }}>Protected by two-factor authentication</div>
        </div>
      </div>
    </div>
  );
}
