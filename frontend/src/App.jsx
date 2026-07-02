import React from 'react';
import { useAuth } from './auth/AuthContext.jsx';
import { useWebmailAuth } from './auth/WebmailAuthContext.jsx';
import { LoginView } from './pages/LoginView.jsx';
import { Shell } from './app/Shell.jsx';
import { WebmailApp } from './app/WebmailApp.jsx';
import { useT } from './i18n/index.jsx';

export function App() {
  const { status: adminStatus } = useAuth();
  const { status: webmailStatus } = useWebmailAuth();
  const t = useT();

  if (adminStatus === 'loading') {
    return (
      <div style={{ display: 'grid', placeItems: 'center', height: '100vh', color: 'var(--muted)', font: '14px system-ui' }}>
        {t('common.loading')}
      </div>
    );
  }
  // Admin access takes precedence: the admin panel embeds the Webmail page and
  // uses the webmail session when present. A webmail-only user gets the standalone
  // webmail app instead.
  if (adminStatus === 'authed') return <Shell />;
  if (webmailStatus === 'authed') return <WebmailApp />;
  return <LoginView />;
}
