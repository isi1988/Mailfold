import React from 'react';
import { cx } from '../../lib/cx.js';

/** Fixed, centred bottom position for a Toast. */
export function Toaster({ className = '', children, ...rest }) {
  return <div className={cx('mf-toaster', className)} {...rest}>{children}</div>;
}
