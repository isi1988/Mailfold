import React from 'react';
import { cx } from '../../lib/cx.js';
import { Icon } from '../atoms/Icon.jsx';

/**
 * Tab strip (+ optional panel). Uncontrolled by default — clicking switches
 * the active tab; pass `active` + `onChange` to control it externally.
 *   items       [{ id, label, icon?, badge?, content? }]
 *   variant     'underline' (default) | 'pill'
 *   defaultActive / active   tab id
 */
export function Tabs({ items = [], active, defaultActive, onChange, variant = 'underline', className = '', ...rest }) {
  const [internal, setInternal] = React.useState(defaultActive ?? (items[0] && items[0].id));
  const current = active !== undefined ? active : internal;
  const select = id => { if (active === undefined) setInternal(id); if (onChange) onChange(id); };
  const activeItem = items.find(it => it.id === current);
  return (
    <div className={cx('mf-tabset', `mf-tabset--${variant}`, className)} {...rest}>
      <div className="mf-tabset__list" role="tablist">
        {items.map(it => {
          const on = it.id === current;
          return (
            <button key={it.id} type="button" role="tab" aria-selected={on}
              className={cx('mf-tabset__tab', on && 'is-active')} onClick={() => select(it.id)}>
              {it.icon && <Icon name={it.icon} size={15} />}
              <span>{it.label}</span>
              {it.badge != null && <span className="mf-tabset__badge">{it.badge}</span>}
            </button>
          );
        })}
      </div>
      {activeItem && activeItem.content != null && (
        <div className="mf-tabset__panel" role="tabpanel">{activeItem.content}</div>
      )}
    </div>
  );
}
