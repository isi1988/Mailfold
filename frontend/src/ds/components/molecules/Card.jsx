import React from 'react';
import { cx } from '../../lib/cx.js';

/** Surface card container. */
export function Card({ pad = false, flush = false, className = '', children, ...rest }) {
  return (
    <div className={cx('mf-card', pad && 'mf-card--pad', flush && 'mf-card--flush', className)} {...rest}>
      {children}
    </div>
  );
}
