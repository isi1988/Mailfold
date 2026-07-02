import React from 'react';
import { cx } from '../../lib/cx.js';
import { Kbd } from '../atoms/Kbd.jsx';
import { Avatar } from '../atoms/Avatar.jsx';
import { Tooltip } from '../atoms/Tooltip.jsx';
import { SearchInput } from '../molecules/SearchInput.jsx';

/** App top bar: command launcher, server status, avatar. */
export function TopBar({ account = {}, server = '', onSearch, onMenu, searchPlaceholder = 'Search or jump to…', serverStatusLabel, accountLabel, className = '', ...rest }) {
  return (
    <header className={cx('mf-topbar', className)} {...rest}>
      {onMenu && (
        <button className="mf-menu-btn" onClick={onMenu} aria-label="Menu">
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="M3 5.5h14M3 10h14M3 14.5h14" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" /></svg>
        </button>
      )}
      <SearchInput button className="mf-topbar__search" placeholder={searchPlaceholder} trailing={<Kbd>⌘K</Kbd>} onClick={onSearch} />
      <div className="mf-topbar__right">
        {server && (
          <Tooltip label={serverStatusLabel ? serverStatusLabel(server) : server} placement="bottom"><div className="mf-server"><span className="mf-dot mf-dot--pulse" />{server}</div></Tooltip>
        )}
        <Tooltip label={account.name || accountLabel || 'Your account'} placement="bottom"><Avatar size={32}>{account.initials || 'JD'}</Avatar></Tooltip>
      </div>
    </header>
  );
}
