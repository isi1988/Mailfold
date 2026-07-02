import React from 'react';
import { cx } from '../../lib/cx.js';

/**
 * Circular initials avatar.
 *   size   24 | 30 | 32 | 34 | 40 | 42 | 54  (px, mapped to a class)
 *   square boolean — rounded-rect instead of circle
 */
export function Avatar({ size = 34, square = false, className = '', children, ...rest }) {
  return (
    <div className={cx('mf-avatar', 'mf-avatar--' + size, square && 'mf-avatar--square', className)} {...rest}>
      {children}
    </div>
  );
}
