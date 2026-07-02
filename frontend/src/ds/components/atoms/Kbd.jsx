import React from 'react';
import { cx } from '../../lib/cx.js';

/** Keyboard key cap, e.g. ⌘K or esc. */
export function Kbd({ className = '', children, ...rest }) {
  return <span className={cx('mf-kbd', className)} {...rest}>{children}</span>;
}
