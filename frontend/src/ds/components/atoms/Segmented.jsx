import React from 'react';
import { cx } from '../../lib/cx.js';

/**
 * Segmented control (Light/Dark, Comfortable/Compact, …).
 *   options   [{ label, value }] or [string]
 *   value     currently-active value/label
 */
export function Segmented({ options = [], value, onSelect, className = '', style, ...rest }) {
  const opts = options.map(o => (typeof o === 'string' ? { label: o, value: o } : o));
  return (
    <div className={cx('mf-seg', className)} style={style} {...rest}>
      {opts.map(o => (
        <span
          key={o.value}
          className={cx('mf-seg__opt', o.value === value && 'mf-seg__opt--active')}
          onClick={onSelect ? () => onSelect(o.value) : undefined}
        >
          {o.label}
        </span>
      ))}
    </div>
  );
}
