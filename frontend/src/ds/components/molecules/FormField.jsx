import React from 'react';
import { cx } from '../../lib/cx.js';
import { Label } from '../atoms/Label.jsx';

/** Label + control wrapper. Pass the control (Input / Select / …) as children. */
export function FormField({ label, children, className = '', ...rest }) {
  return (
    <div className={cx('mf-field', className)} {...rest}>
      {label && <Label>{label}</Label>}
      {children}
    </div>
  );
}
