import React from 'react';
import { cx } from '../../lib/cx.js';
import { Icon } from '../atoms/Icon.jsx';

/**
 * Search field.
 *   sm        boolean — compact variant (list heads)
 *   button    boolean — render as a clickable pill (topbar ⌘K launcher)
 *   trailing  node    — e.g. a <Kbd>⌘K</Kbd>
 */
export function SearchInput({ placeholder = 'Search…', value, onChange, sm = false, button = false, trailing, className = '', ...rest }) {
  const cls = cx('mf-search', sm && 'mf-search--sm', className);
  const icon = <Icon name="search" size={sm ? 14 : 15} />;
  if (button) {
    return (
      <div className={cls} {...rest}>
        {icon}
        <span style={{ flex: 1 }}>{placeholder}</span>
        {trailing}
      </div>
    );
  }
  return (
    <div className={cls} {...rest}>
      {icon}
      <input className="mf-search__input" placeholder={placeholder} value={value} onChange={onChange} />
      {trailing}
    </div>
  );
}
