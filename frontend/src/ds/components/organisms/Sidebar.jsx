import React from 'react';
import { cx } from '../../lib/cx.js';
import { Logo } from '../atoms/Logo.jsx';
import { Segmented } from '../atoms/Segmented.jsx';
import { Avatar } from '../atoms/Avatar.jsx';
import { Icon } from '../atoms/Icon.jsx';
import { Divider } from '../atoms/Divider.jsx';
import { NavItem, NavGroup } from '../molecules/NavItem.jsx';

/**
 * Left sidebar: brand, nav, theme switch, user.
 *   nav       [{key,label,badge} | {group}]
 *   current   active route key
 *   theme     'light' | 'dark'
 *   account   {name, role, initials}
 */
export function Sidebar({ nav = [], current = 'dashboard', theme = 'light', account = {}, onNavigate, onTheme, onLogout, themeOptions, className = '', ...rest }) {
  const segments = themeOptions || [{ label: 'Light', value: 'light' }, { label: 'Dark', value: 'dark' }];
  return (
    <aside className={cx('mf-sidebar', className)} {...rest}>
      <div className="mf-sidebar__brand"><Logo size="sm" /></div>
      <nav className="mf-nav">
        {nav.map((n, i) => n.group
          ? <NavGroup key={'g' + i}>{n.group}</NavGroup>
          : <NavItem key={n.key} label={n.label} badge={n.badge} active={n.key === current} onClick={onNavigate ? () => onNavigate(n.key) : undefined} />
        )}
      </nav>
      <div className="mf-sidebar__foot">
        <Segmented options={segments} value={theme} onSelect={onTheme} />
        <Divider />
        <div className="mf-user">
          <Avatar size={30}>{account.initials || 'JD'}</Avatar>
          <div>
            <div className="mf-user__name">{account.name || 'Jamie Doe'}</div>
            <div className="mf-user__meta">{account.role || 'Admin · Acme'}</div>
          </div>
          <div
            className="mf-user__logout"
            role={onLogout ? 'button' : undefined}
            title={account.logoutLabel}
            style={onLogout ? { cursor: 'pointer' } : undefined}
            onClick={onLogout}
          >
            <Icon name="logout" size={16} />
          </div>
        </div>
      </div>
    </aside>
  );
}
