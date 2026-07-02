import React from 'react';
import { cx } from '../../lib/cx.js';

/**
 * Progress / quota bar.
 *   pct   0–100
 *   tone  'accent' (default) | 'amber' | 'red'  — or pass auto to derive from pct
 *   lg    boolean — taller track
 */
export function ProgressBar({ pct = 0, tone = 'accent', auto = false, lg = false, className = '', ...rest }) {
  const t = auto ? (pct > 85 ? 'red' : pct > 60 ? 'amber' : 'accent') : tone;
  return (
    <div className={cx('mf-bar', lg && 'mf-bar--lg', className)} {...rest}>
      <span className={cx('mf-bar__fill', t !== 'accent' && 'mf-bar__fill--' + t)} style={{ width: pct + '%' }} />
    </div>
  );
}
