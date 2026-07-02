import React from 'react';
import { Logo } from '../ds/components/atoms/Logo.jsx';
import { WebmailPage } from '../pages/WebmailPage.jsx';

// Standalone chrome for a webmail-only user (no admin nav). The Webmail page owns
// the session and its own sign-out.
export function WebmailApp() {
  return (
    <div style={{ height: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--bg)' }}>
      <header style={{ display: 'flex', alignItems: 'center', padding: '12px 22px', borderBottom: '1px solid var(--hair)', flex: 'none' }}>
        <Logo size="sm" />
      </header>
      <div style={{ flex: 1, minHeight: 0, padding: 16 }}>
        <WebmailPage />
      </div>
    </div>
  );
}
