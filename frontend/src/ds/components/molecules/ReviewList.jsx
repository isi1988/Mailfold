import React from 'react';
import { cx } from '../../lib/cx.js';

/** One key/value line inside a ReviewList. */
export function ReviewRow({ label, children }) {
  return (
    <div className="mf-review__row">
      <span className="mf-review__key">{label}</span>
      <span className="mf-review__val">{children}</span>
    </div>
  );
}

/** Summary panel used on the final step of every wizard. */
export function ReviewList({ children, className = '', ...rest }) {
  return <div className={cx('mf-review', className)} {...rest}>{children}</div>;
}
