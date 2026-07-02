import React from 'react';
import { cx } from '../../lib/cx.js';
import { Sidebar } from './Sidebar.jsx';
import { TopBar } from './TopBar.jsx';

/**
 * Full application chrome: sidebar + top bar + scrolling content.
 * Page content goes in `children`.
 *   wide  boolean — remove the 1180px content cap (used by Webmail)
 */
export function AppShell({ nav, current, theme = 'light', account, server, serverStatusLabel, accountLabel, onNavigate, onTheme, onSearch, onLogout, searchPlaceholder, themeOptions, wide = false, children }) {
  const [menuOpen, setMenuOpen] = React.useState(false);
  return (
    <div className="mf-app">
      <Sidebar
        nav={nav}
        current={current}
        theme={theme}
        account={account}
        onNavigate={key => { if (onNavigate) onNavigate(key); setMenuOpen(false); }}
        onTheme={onTheme}
        onLogout={onLogout}
        themeOptions={themeOptions}
        className={menuOpen ? 'mf-sidebar--open' : ''}
      />
      {menuOpen && <div className="mf-sidebar-backdrop" onClick={() => setMenuOpen(false)} />}
      <div className="mf-main">
        <TopBar account={account} server={server} serverStatusLabel={serverStatusLabel} accountLabel={accountLabel} onSearch={onSearch} searchPlaceholder={searchPlaceholder} onMenu={() => setMenuOpen(o => !o)} />
        <div className={cx('mf-content', wide && 'mf-content--wide')}>
          <div className="mf-content__inner">{children}</div>
        </div>
      </div>
    </div>
  );
}
