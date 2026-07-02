import React from 'react';
import { cx } from '../../lib/cx.js';

// Shares the grid column template between the header and every row.
const TableCtx = React.createContext('');

/**
 * Table shell.
 *   columns  [{ label, w, align }]   — w is a grid track (e.g. '2fr', '90px')
 * Rows are <TableRow> children.
 */
export function Table({ columns = [], className = '', children, ...rest }) {
  const template = columns.map(c => c.w).join(' ');
  return (
    <div className={cx('mf-table', className)} {...rest}>
      <div className="mf-table__head" style={{ gridTemplateColumns: template }}>
        {columns.map((c, i) => <span key={i} style={c.align ? { textAlign: c.align } : undefined}>{c.label}</span>)}
      </div>
      <TableCtx.Provider value={template}>{children}</TableCtx.Provider>
    </div>
  );
}

/** A row that inherits the parent Table's column template. */
export function TableRow({ onClick, plain = false, className = '', children, ...rest }) {
  const template = React.useContext(TableCtx);
  return (
    <div
      className={cx('mf-table__row', plain && 'mf-table__row--static', className)}
      style={{ gridTemplateColumns: template }}
      onClick={onClick}
      {...rest}
    >
      {children}
    </div>
  );
}
