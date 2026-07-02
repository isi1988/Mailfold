import React from 'react';
import { cx } from '../../lib/cx.js';

/** Visual checkbox (unchecked square). Presentational only. */
export function Checkbox({ className = '', ...rest }) {
  return <span className={cx('mf-check', className)} {...rest} />;
}
