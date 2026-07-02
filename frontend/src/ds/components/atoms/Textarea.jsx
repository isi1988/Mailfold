import React from 'react';
import { cx } from '../../lib/cx.js';

/** Multi-line text input (compose body, etc). */
export function Textarea({ className = '', ...rest }) {
  return <textarea className={cx('mf-textarea', className)} {...rest} />;
}
