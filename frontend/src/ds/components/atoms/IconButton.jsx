import React from 'react';
import { cx } from '../../lib/cx.js';

/** A 32×32 square icon button with a hover wash (webmail toolbar, etc). */
export function IconButton({ className = '', children, ...rest }) {
  return (
    <button className={cx('mf-icon-btn', className)} {...rest}>
      {children}
    </button>
  );
}
