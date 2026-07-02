import React from 'react';
import { cx } from '../../lib/cx.js';

/**
 * Tag / chip (allowlist entries, alias tags, etc).
 *   tone  'neutral' | 'allow' | 'block' | 'accent' | 'dashed'
 */
export function Chip({ tone = 'neutral', className = '', children, ...rest }) {
  return (
    <span className={cx('mf-chip', tone !== 'neutral' && 'mf-chip--' + tone, className)} {...rest}>
      {children}
    </span>
  );
}
