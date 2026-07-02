import React, { useState, useEffect } from 'react';
import { Routes, Route, Navigate, useNavigate, useLocation } from 'react-router-dom';
import { AppShell } from '../ds/components/organisms/AppShell.jsx';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { NAV } from './nav.js';
import { useAuth } from '../auth/AuthContext.jsx';
import { getTheme, applyTheme } from './theme.js';
import { initials } from '../ds/data/sample.js';
import { useT } from '../i18n/index.jsx';
import { MailboxesPage } from '../pages/MailboxesPage.jsx';
import { DomainsPage } from '../pages/DomainsPage.jsx';
import { AliasesPage } from '../pages/AliasesPage.jsx';

// Temporary placeholder until each page is wired to live data.
function Placeholder({ title }) {
  return <PageHeader title={title} sub="Wiring in progress…" />;
}

// The authenticated application chrome: one AppShell (sidebar + top bar) with the
// routed page content inside. Nav keys map directly to routes.
export function Shell() {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, logout } = useAuth();
  const t = useT();
  const [theme, setTheme] = useState(getTheme());

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  const current = location.pathname.split('/')[1] || 'dashboard';
  const wide = current === 'webmail';
  const account = {
    name: user || 'Admin',
    role: t('shell.role'),
    initials: initials(user || 'Admin'),
    email: user,
    logoutLabel: t('shell.logout'),
  };
  // Resolve nav labels/group headers through i18n so they follow the language.
  const nav = NAV.map(n =>
    n.group ? { group: t('nav.group.' + n.group) } : { key: n.key, label: t('nav.' + n.key), badge: n.badge },
  );
  const themeOptions = [
    { label: t('shell.theme.light'), value: 'light' },
    { label: t('shell.theme.dark'), value: 'dark' },
  ];

  return (
    <AppShell
      nav={nav}
      current={current}
      theme={theme}
      account={account}
      wide={wide}
      searchPlaceholder={t('shell.searchPlaceholder')}
      themeOptions={themeOptions}
      onNavigate={key => navigate('/' + key)}
      onTheme={setTheme}
      onSearch={() => {}}
      onLogout={logout}
    >
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/dashboard" element={<Placeholder title={t('nav.dashboard')} />} />
        <Route path="/mailboxes" element={<MailboxesPage />} />
        <Route path="/domains" element={<DomainsPage />} />
        <Route path="/aliases" element={<AliasesPage />} />
        <Route path="/queue" element={<Placeholder title={t('nav.queue')} />} />
        <Route path="/quarantine" element={<Placeholder title={t('nav.quarantine')} />} />
        <Route path="/spam" element={<Placeholder title={t('nav.spam')} />} />
        <Route path="/syncjobs" element={<Placeholder title={t('nav.syncjobs')} />} />
        <Route path="/logs" element={<Placeholder title={t('nav.logs')} />} />
        <Route path="/webmail" element={<Placeholder title={t('nav.webmail')} />} />
        <Route path="/settings" element={<Placeholder title={t('nav.settings')} />} />
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </AppShell>
  );
}
