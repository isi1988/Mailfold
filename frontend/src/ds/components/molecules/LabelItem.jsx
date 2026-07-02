import React from 'react';
import { cx } from '../../lib/cx.js';

/** Webmail label row — a coloured swatch + name. */
export function LabelItem({ color, label, className = '', ...rest }) {
  return (
    <div className={cx('mf-label-item', className)} {...rest}>
      <span className="mf-label-item__swatch" style={{ background: color }} />
      {label}
    </div>
  );
}
