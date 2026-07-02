import React from 'react';
import { cx } from '../../lib/cx.js';

/** Section heading inside the sidebar nav. */
export function NavGroup({ children }) {
  return <div className="mf-nav__group">{children}</div>;
}

/** A single sidebar nav row. */
export function NavItem({ label, badge, active = false, onClick, className = '', ...rest }) {
  return (
    <div className={cx('mf-nav__item', active && 'mf-nav__item--active', className)} onClick={onClick} {...rest}>
      <span className="mf-nav__dot" />
      {label}
      {badge != null && <span className="mf-nav__badge">{badge}</span>}
    </div>
  );
}
