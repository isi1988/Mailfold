import React from 'react';
import { useAuth } from './auth/AuthContext.jsx';
import { useWebmailAuth } from './auth/WebmailAuthContext.jsx';
import { useDomainAdminAuth } from './auth/DomainAdminAuthContext.jsx';
import { LoginView } from './pages/LoginView.jsx';
import { ResetPasswordPage } from './pages/ResetPasswordPage.jsx';
import { Shell } from './app/Shell.jsx';
import { WebmailApp } from './app/WebmailApp.jsx';
import { DomainAdminShell } from './app/DomainAdminShell.jsx';
import { useT } from './i18n/index.jsx';

export function App() {
  const { status: adminStatus } = useAuth();
  const { status: webmailStatus } = useWebmailAuth();
  const { status: domainAdminStatus } = useDomainAdminAuth();
  const t = useT();

  // /reset is reached straight from an emailed link, before any session exists —
  // it must render regardless of auth state, so it is checked ahead of every
  // other screen below.
  if (window.location.pathname === '/reset') {
    return <ResetPasswordPage />;
  }

  if (adminStatus === 'loading') {
    return (
      <div style={{ display: 'grid', placeItems: 'center', height: '100vh', color: 'var(--muted)', font: '14px system-ui' }}>
        {t('common.loading')}
      </div>
    );
  }
  // Admin access takes precedence: the admin panel embeds the Webmail page and
  // uses the webmail session when present. A domain admin (scoped to a subset
  // of domains, distinct from both the super-admin and a webmail mailbox) gets
  // its own minimal shell. A webmail-only user gets the standalone webmail app.
  if (adminStatus === 'authed') return <Shell />;
  if (domainAdminStatus === 'authed') return <DomainAdminShell />;
  if (webmailStatus === 'authed') return <WebmailApp />;
  return <LoginView />;
}
