import React from 'react';
import { cx } from '../../lib/cx.js';

/**
 * Toggle switch (presentational).
 *   on  boolean
 */
export function Toggle({ on = false, className = '', ...rest }) {
  return (
    <div className={cx('mf-toggle', on && 'mf-toggle--on', className)} role="switch" aria-checked={on} {...rest}>
      <span className="mf-toggle__knob" />
    </div>
  );
}
