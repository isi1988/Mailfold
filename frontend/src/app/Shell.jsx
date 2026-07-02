import React, { useState, useEffect } from 'react';
import { Routes, Route, Navigate, useNavigate, useLocation } from 'react-router-dom';
import { AppShell } from '../ds/components/organisms/AppShell.jsx';
import { NAV } from './nav.js';
import { useAuth } from '../auth/AuthContext.jsx';
import { getTheme, applyTheme } from './theme.js';
import { initials } from '../ds/data/sample.js';
import { useT } from '../i18n/index.jsx';
import { useApi } from '../lib/useApi.js';
import { MailboxesPage } from '../pages/MailboxesPage.jsx';
import { DomainsPage } from '../pages/DomainsPage.jsx';
import { AliasesPage } from '../pages/AliasesPage.jsx';
import { DashboardPage } from '../pages/DashboardPage.jsx';
import { QueuePage } from '../pages/QueuePage.jsx';
import { QuarantinePage } from '../pages/QuarantinePage.jsx';
import { SpamPage } from '../pages/SpamPage.jsx';
import { SyncJobsPage } from '../pages/SyncJobsPage.jsx';
import { LogsPage } from '../pages/LogsPage.jsx';
import { SettingsPage } from '../pages/SettingsPage.jsx';
import { WebmailPage } from '../pages/WebmailPage.jsx';
import { ApiKeysPage } from '../pages/ApiKeysPage.jsx';

// The authenticated application chrome: one AppShell (sidebar + top bar) with the
// routed page content inside. Nav keys map directly to routes.
export function Shell() {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, logout } = useAuth();
  const t = useT();
  const [theme, setTheme] = useState(getTheme());
  const { data: serverStatus } = useApi('/api/status/server');
  const serverName = (serverStatus && serverStatus.name) || '';

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
      server={serverName}
      serverStatusLabel={name => t('topbar.serverStatus', { server: name })}
      accountLabel={t('topbar.account')}
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
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/mailboxes" element={<MailboxesPage />} />
        <Route path="/domains" element={<DomainsPage />} />
        <Route path="/aliases" element={<AliasesPage />} />
        <Route path="/queue" element={<QueuePage />} />
        <Route path="/quarantine" element={<QuarantinePage />} />
        <Route path="/spam" element={<SpamPage />} />
        <Route path="/syncjobs" element={<SyncJobsPage />} />
        <Route path="/logs" element={<LogsPage />} />
        <Route path="/webmail" element={<WebmailPage />} />
        <Route path="/apikeys" element={<ApiKeysPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </AppShell>
  );
}
