import React from 'react';
import { cx } from '../../lib/cx.js';

/** Hairline divider. Set vertical for a 1px column rule. */
export function Divider({ soft = false, vertical = false, className = '', style, ...rest }) {
  return (
    <div
      className={cx('mf-divider', soft && 'mf-divider--soft', className)}
      style={vertical ? { width: 1, height: 'auto', alignSelf: 'stretch', ...style } : style}
      {...rest}
    />
  );
}
