import React from 'react';
import { cx } from '../../lib/cx.js';

/** Field label. */
export function Label({ strong = false, className = '', children, ...rest }) {
  return (
    <label className={cx('mf-label', strong && 'mf-label--strong', className)} {...rest}>
      {children}
    </label>
  );
}
