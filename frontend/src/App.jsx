import React from 'react';
import { useAuth } from './auth/AuthContext.jsx';
import { LoginView } from './pages/LoginView.jsx';
import { Shell } from './app/Shell.jsx';

export function App() {
  const { status } = useAuth();

  if (status === 'loading') {
    return (
      <div style={{ display: 'grid', placeItems: 'center', height: '100vh', color: 'var(--muted)', font: '14px system-ui' }}>
        Loading…
      </div>
    );
  }
  if (status !== 'authed') return <LoginView />;
  return <Shell />;
}
