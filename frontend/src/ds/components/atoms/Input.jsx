import React from 'react';
import { cx } from '../../lib/cx.js';

/**
 * Text input.
 *   mono      boolean — monospace value
 *   size      'md' | 'lg'
 *   align     'right' to right-align (numeric)
 *   readonly  boolean — muted, non-editable look
 */
export function Input({ mono = false, size = 'md', align, readonly = false, className = '', ...rest }) {
  return (
    <input
      readOnly={readonly}
      className={cx(
        'mf-input',
        mono && 'mf-input--mono',
        size === 'lg' && 'mf-input--lg',
        align === 'right' && 'mf-input--right',
        readonly && 'mf-input--readonly',
        className,
      )}
      {...rest}
    />
  );
}
