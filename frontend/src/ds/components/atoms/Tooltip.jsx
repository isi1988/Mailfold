import React from 'react';
import { cx } from '../../lib/cx.js';

/**
 * CSS-only tooltip (shows on hover / keyboard focus — no runtime JS).
 *   label      tooltip text
 *   placement  'top' | 'bottom' | 'left' | 'right'   (default 'top')
 *   children   the trigger element
 */
export function Tooltip({ label, placement = 'top', children, className = '', ...rest }) {
  return (
    <span className={cx('mf-tip', `mf-tip--${placement}`, className)} tabIndex={0} {...rest}>
      {children}
      <span className="mf-tip__bubble" role="tooltip">
        {label}
        <i className="mf-tip__arrow" aria-hidden="true" />
      </span>
    </span>
  );
}
