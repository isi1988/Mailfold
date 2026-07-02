import React from 'react';
import { cx } from '../../lib/cx.js';
import { Icon } from '../atoms/Icon.jsx';

/** Confirmation toast card. Wrap in <Toaster> to position it. */
export function Toast({ title, msg, onDismiss, className = '', ...rest }) {
  return (
    <div className={cx('mf-toast', className)} {...rest}>
      <span className="mf-toast__check"><Icon name="check" size={13} style={{ color: '#fff' }} /></span>
      <div className="mf-stack" style={{ gap: 1 }}>
        <span className="mf-toast__title">{title}</span>
        {msg && <span className="mf-toast__msg">{msg}</span>}
      </div>
      {onDismiss && <span className="mf-toast__x" onClick={onDismiss}><Icon name="close-sm" size={14} /></span>}
    </div>
  );
}
