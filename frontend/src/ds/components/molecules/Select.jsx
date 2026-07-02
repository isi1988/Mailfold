import React from 'react';
import { cx } from '../../lib/cx.js';
import { Icon } from '../atoms/Icon.jsx';

/**
 * Custom select. Presentational — pass `open` to render the menu for reference.
 *   value     current label
 *   options   [string]
 *   open      boolean — show the dropdown
 *   mono      boolean — monospace value
 *   search    boolean — show a search field at the top of the menu
 */
export function Select({ value, options = [], open = false, mono = false, search = false, className = '', ...rest }) {
  return (
    <div className={cx('mf-select', open && 'mf-select--open', mono && 'mf-select--mono', className)} {...rest}>
      <div className="mf-select__control">
        <span className="mf-select__value">{value}</span>
        <Icon name="chevron-down" size={16} className="mf-select__caret" />
      </div>
      {open && (
        <div className="mf-select__menu">
          {search && (
            <div className="mf-select__search">
              <Icon name="search" size={14} style={{ color: 'var(--faint)' }} />
              <input placeholder="Search…" />
            </div>
          )}
          {options.map(o => (
            <div key={o} className={cx('mf-select__opt', o === value && 'mf-select__opt--active')}>{o}</div>
          ))}
        </div>
      )}
    </div>
  );
}
