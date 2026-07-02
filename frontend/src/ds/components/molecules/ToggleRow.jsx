import React from 'react';
import { cx } from '../../lib/cx.js';
import { Toggle } from '../atoms/Toggle.jsx';

/**
 * A row with title + description on the left and a control on the right.
 * Default control is a Toggle; pass `control` for a badge, button or value
 * (used across Spam rules and Settings).
 */
export function ToggleRow({ title, desc, on, control, flush = false, className = '', ...rest }) {
  return (
    <div className={cx('mf-toggle-row', flush && 'mf-toggle-row--flush', className)} {...rest}>
      <div className="mf-toggle-row__text">
        <div className="mf-toggle-row__title">{title}</div>
        {desc && <div className="mf-toggle-row__desc">{desc}</div>}
      </div>
      {control !== undefined ? control : <Toggle on={on} />}
    </div>
  );
}
