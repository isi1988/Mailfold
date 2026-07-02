import React from 'react';
import { cx } from '../../lib/cx.js';

/**
 * Segmented filter tabs (All / Active / Disabled).
 *   options  [string] or [{label, value}]
 *   value    active value
 */
export function FilterTabs({ options = [], value, onSelect, className = '', ...rest }) {
  const opts = options.map(o => (typeof o === 'string' ? { label: o, value: o } : o));
  return (
    <div className={cx('mf-tabs', className)} {...rest}>
      {opts.map(o => (
        <span
          key={o.value}
          className={cx('mf-tab', o.value === value && 'mf-tab--active')}
          onClick={onSelect ? () => onSelect(o.value) : undefined}
        >
          {o.label}
        </span>
      ))}
    </div>
  );
}
