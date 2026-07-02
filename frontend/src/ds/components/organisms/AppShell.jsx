import React from 'react';
import { cx } from '../../lib/cx.js';
import { Sidebar } from './Sidebar.jsx';
import { TopBar } from './TopBar.jsx';

/**
 * Full application chrome: sidebar + top bar + scrolling content.
 * Page content goes in `children`.
 *   wide  boolean — remove the 1180px content cap (used by Webmail)
 */
export function AppShell({ nav, current, theme = 'light', account, server, onNavigate, onTheme, onSearch, onLogout, searchPlaceholder, themeOptions, wide = false, children }) {
  return (
    <div className="mf-app">
      <Sidebar nav={nav} current={current} theme={theme} account={account} onNavigate={onNavigate} onTheme={onTheme} onLogout={onLogout} themeOptions={themeOptions} />
      <div className="mf-main">
        <TopBar account={account} server={server} onSearch={onSearch} searchPlaceholder={searchPlaceholder} />
        <div className={cx('mf-content', wide && 'mf-content--wide')}>
          <div className="mf-content__inner">{children}</div>
        </div>
      </div>
    </div>
  );
}
