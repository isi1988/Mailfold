import React from 'react';
import { cx } from '../../lib/cx.js';

/**
 * Small status dot.
 *   tone   'green' (default) | 'amber' | 'red' | 'accent' | 'faint'
 *   pulse  boolean — gentle heartbeat (live indicators)
 */
export function StatusDot({ tone = 'green', pulse = false, className = '', ...rest }) {
  return <span className={cx('mf-dot', tone !== 'green' && 'mf-dot--' + tone, pulse && 'mf-dot--pulse', className)} {...rest} />;
}
