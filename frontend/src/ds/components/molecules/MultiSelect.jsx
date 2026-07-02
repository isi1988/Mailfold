import React from 'react';
import { cx } from '../../lib/cx.js';
import { Token } from '../atoms/Token.jsx';
import { Avatar } from '../atoms/Avatar.jsx';

// 2-letter initials from an email local-part, e.g. "a.ruiz@x" -> "AR"
function addrInitials(addr) {
  const name = String(addr).split('@')[0];
  const parts = name.split(/[._+-]+/).filter(Boolean);
  const ini = parts.length >= 2 ? parts[0][0] + parts[1][0] : name.slice(0, 2);
  return ini.toUpperCase();
}

/**
 * Recipient multi-select (tokens + input). Presentational.
 *   values       [string] — selected addresses
 *   options      [string] — suggestions (selected are filtered out of the menu)
 *   open         boolean  — show the suggestion menu
 *   flat         boolean  — borderless variant (compose To/Cc)
 *   placeholder  string
 *   onRemove     (value) => void
 */
export function MultiSelect({ values = [], options = [], open = false, flat = false, placeholder = 'Add a recipient…', onRemove, className = '', ...rest }) {
  const remaining = options.filter(o => !values.includes(o));
  return (
    <div className={cx('mf-multiselect', open && 'mf-multiselect--open', flat && 'mf-multiselect--flat', className)} {...rest}>
      <div className="mf-multiselect__box">
        {values.map(v => <Token key={v} label={v} onRemove={onRemove ? () => onRemove(v) : undefined} />)}
        <input className="mf-multiselect__input" placeholder={values.length ? '' : placeholder} />
      </div>
      {open && (
        <div className="mf-select__menu">
          {remaining.map(o => (
            <div key={o} className="mf-select__opt">
              <Avatar size={24}>{addrInitials(o)}</Avatar>
              <span className="mf-truncate" style={{ flex: 1, fontFamily: 'var(--font-mono)' }}>{o}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
