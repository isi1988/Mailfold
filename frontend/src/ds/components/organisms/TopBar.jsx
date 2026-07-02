import React from 'react';
import { cx } from '../../lib/cx.js';
import { Icon } from '../atoms/Icon.jsx';
import { Kbd } from '../atoms/Kbd.jsx';
import { Avatar } from '../atoms/Avatar.jsx';
import { Tooltip } from '../atoms/Tooltip.jsx';
import { SearchInput } from '../molecules/SearchInput.jsx';

/** App top bar: command launcher, docs, notifications, server status, avatar. */
export function TopBar({ account = {}, server = 'mail.acme.io', onSearch, onMenu, searchPlaceholder = 'Search or jump to…', className = '', ...rest }) {
  return (
    <header className={cx('mf-topbar', className)} {...rest}>
      {onMenu && (
        <button className="mf-menu-btn" onClick={onMenu} aria-label="Menu">
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="M3 5.5h14M3 10h14M3 14.5h14" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" /></svg>
        </button>
      )}
      <SearchInput button className="mf-topbar__search" placeholder={searchPlaceholder} trailing={<Kbd>⌘K</Kbd>} onClick={onSearch} />
      <div className="mf-topbar__right">
        <Tooltip label="Open documentation" placement="bottom"><span className="mf-u-muted" style={{ fontSize: 13, cursor: 'pointer' }}>Docs</span></Tooltip>
        <Tooltip label="Notifications" placement="bottom"><div className="mf-bell"><Icon name="bell" size={17} /><span className="mf-bell__dot" /></div></Tooltip>
        <Tooltip label={server + ' · operational'} placement="bottom"><div className="mf-server"><span className="mf-dot mf-dot--pulse" />{server}</div></Tooltip>
        <Tooltip label={account.name || 'Your account'} placement="bottom"><Avatar size={32}>{account.initials || 'JD'}</Avatar></Tooltip>
      </div>
    </header>
  );
}
