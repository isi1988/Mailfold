import React from 'react';
import { cx } from '../../lib/cx.js';
import { Logo } from '../atoms/Logo.jsx';
import { Segmented } from '../atoms/Segmented.jsx';
import { Avatar } from '../atoms/Avatar.jsx';
import { Icon } from '../atoms/Icon.jsx';
import { Divider } from '../atoms/Divider.jsx';
import { NavItem, NavGroup } from '../molecules/NavItem.jsx';

// CollapsibleGroup renders a clickable section header that shows/hides its items.
// It opens automatically when the active route is one of its items.
function CollapsibleGroup({ label, items = [], current, onNavigate }) {
  const containsCurrent = items.some(it => it.key === current);
  const [open, setOpen] = React.useState(containsCurrent);
  React.useEffect(() => { if (containsCurrent) setOpen(true); }, [containsCurrent]);
  return (
    <>
      <div
        className="mf-nav__group mf-nav__group--toggle"
        role="button"
        onClick={() => setOpen(o => !o)}
        style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}
      >
        <span>{label}</span>
        <Icon name="chevron-right" size={12} style={{ transform: open ? 'rotate(90deg)' : 'none', transition: 'transform .15s', opacity: 0.6 }} />
      </div>
      {open && items.map(it => (
        <NavItem key={it.key} label={it.label} badge={it.badge} active={it.key === current} onClick={onNavigate ? () => onNavigate(it.key) : undefined} />
      ))}
    </>
  );
}

/**
 * Left sidebar: brand, nav, theme switch, user.
 *   nav       [{key,label,badge} | {group} | {collapsibleGroup,label,items}]
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
        {nav.map((n, i) => {
          if (n.collapsibleGroup) {
            return <CollapsibleGroup key={'c' + i} label={n.label} items={n.items} current={current} onNavigate={onNavigate} />;
          }
          if (n.group) return <NavGroup key={'g' + i}>{n.group}</NavGroup>;
          return <NavItem key={n.key} label={n.label} badge={n.badge} active={n.key === current} onClick={onNavigate ? () => onNavigate(n.key) : undefined} />;
        })}
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
