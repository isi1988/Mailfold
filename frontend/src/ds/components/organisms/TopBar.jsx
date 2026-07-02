import React from 'react';
import { cx } from '../../lib/cx.js';
import { Icon } from '../atoms/Icon.jsx';
import { Kbd } from '../atoms/Kbd.jsx';
import { Avatar } from '../atoms/Avatar.jsx';
import { SearchInput } from '../molecules/SearchInput.jsx';

/** App top bar: command launcher, docs, notifications, server status, avatar. */
export function TopBar({ account = {}, server = 'mail.acme.io', onSearch, searchPlaceholder = 'Search or jump to…', className = '', ...rest }) {
  return (
    <header className={cx('mf-topbar', className)} {...rest}>
      <SearchInput button className="mf-topbar__search" placeholder={searchPlaceholder} trailing={<Kbd>⌘K</Kbd>} onClick={onSearch} />
      <div className="mf-topbar__right">
        <span className="mf-u-muted" style={{ fontSize: 13, cursor: 'pointer' }}>Docs</span>
        <div className="mf-bell"><Icon name="bell" size={17} /><span className="mf-bell__dot" /></div>
        <div className="mf-server"><span className="mf-dot mf-dot--pulse" />{server}</div>
        <Avatar size={32}>{account.initials || 'JD'}</Avatar>
      </div>
    </header>
  );
}
