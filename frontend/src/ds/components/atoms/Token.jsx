import React from 'react';
import { cx } from '../../lib/cx.js';
import { Icon } from './Icon.jsx';

/**
 * Removable token used inside the multi-select (accent-tinted).
 *   label     string
 *   onRemove  () => void
 */
export function Token({ label, onRemove, className = '', ...rest }) {
  return (
    <span className={cx('mf-token', className)} {...rest}>
      <span className="mf-token__label">{label}</span>
      <span className="mf-token__x" onClick={onRemove} role="button" aria-label={'Remove ' + label}>
        <Icon name="close-sm" size={12} />
      </span>
    </span>
  );
}
