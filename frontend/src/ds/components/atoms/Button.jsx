import React from 'react';
import { cx } from '../../lib/cx.js';

/**
 * Button.
 *   variant  'primary' | 'secondary' | 'ghost' | 'danger' | 'danger-solid' | 'link'
 *   size     'sm' | 'md' | 'lg'
 *   block    boolean — full width
 */
export function Button({ variant = 'secondary', size = 'md', block = false, className = '', children, ...rest }) {
  return (
    <button
      className={cx('mf-btn', 'mf-btn--' + variant, size !== 'md' && 'mf-btn--' + size, block && 'mf-btn--block', className)}
      {...rest}
    >
      {children}
    </button>
  );
}
