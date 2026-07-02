import React from 'react';
import { cx } from '../../lib/cx.js';
import { Icon } from '../atoms/Icon.jsx';
import { Kbd } from '../atoms/Kbd.jsx';

/**
 * ⌘K command palette.
 *   results  [{ label, hint, onClick }]
 *   query    prefilled query text
 *   onClose  () => void  (backdrop click)
 */
export function CommandPalette({ results = [], query = '', onClose, className = '', ...rest }) {
  return (
    <div className="mf-palette-overlay" onClick={onClose}>
      <div className={cx('mf-palette', className)} onClick={e => e.stopPropagation()} {...rest}>
        <div className="mf-palette__head">
          <Icon name="search" size={16} style={{ color: 'var(--faint)' }} />
          <input className="mf-palette__input" placeholder="Jump to a page…" defaultValue={query} />
          <Kbd>esc</Kbd>
        </div>
        <div className="mf-palette__list">
          {results.map((r, i) => (
            <div key={i} className="mf-palette__item" onClick={r.onClick}>
              <span style={{ width: 5, height: 5, borderRadius: 1.5, background: 'var(--accent)' }} />
              {r.label}
              <span className="mf-palette__hint">{r.hint || 'Go'}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
