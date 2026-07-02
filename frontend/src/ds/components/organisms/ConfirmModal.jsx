import React from 'react';
import { cx } from '../../lib/cx.js';
import { Button } from '../atoms/Button.jsx';

/**
 * Centred confirmation dialog.
 *   title, msg, cta
 *   danger   boolean — red confirm button
 *   onCancel, onConfirm
 */
export function ConfirmModal({ title, msg, cta = 'Confirm', danger = false, onCancel, onConfirm, className = '', ...rest }) {
  return (
    <div className="mf-overlay mf-overlay--center" onClick={onCancel}>
      <div className={cx('mf-dialog', className)} onClick={e => e.stopPropagation()} {...rest}>
        <div className="mf-dialog__body">
          <div className="mf-dialog__title">{title}</div>
          <div className="mf-dialog__msg">{msg}</div>
        </div>
        <div className="mf-dialog__foot">
          <Button variant="secondary" onClick={onCancel}>Cancel</Button>
          <Button variant={danger ? 'danger-solid' : 'primary'} onClick={onConfirm}>{cta}</Button>
        </div>
      </div>
    </div>
  );
}
