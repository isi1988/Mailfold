import React from 'react';
import { cx } from '../../lib/cx.js';

/**
 * Status pill.
 *   tone  'green' | 'amber' | 'blue' | 'red' | 'neutral'
 */
export function Pill({ tone = 'neutral', className = '', children, ...rest }) {
  return (
    <span className={cx('mf-pill', 'mf-pill--' + tone, className)} {...rest}>
      {children}
    </span>
  );
}
