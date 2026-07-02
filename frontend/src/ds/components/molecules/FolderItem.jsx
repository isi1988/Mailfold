import React from 'react';
import { cx } from '../../lib/cx.js';

/** Webmail folder row. Pass an `icon` node; `count` shows a trailing tally. */
export function FolderItem({ icon, label, count, active = false, nested = false, className = '', children, ...rest }) {
  return (
    <div className={cx('mf-folder', active && 'mf-folder--active', nested && 'mf-folder--nested', className)} {...rest}>
      {icon}
      {label || children}
      {count != null && <span className="mf-folder__count">{count}</span>}
    </div>
  );
}
