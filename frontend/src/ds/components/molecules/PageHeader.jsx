import React from 'react';
import { cx } from '../../lib/cx.js';

/** Page title + subtitle + right-aligned actions. */
export function PageHeader({ title, sub, actions, className = '', ...rest }) {
  return (
    <div className={cx('mf-page-head', className)} {...rest}>
      <div>
        <h1 className="mf-page-head__title">{title}</h1>
        {sub && <div className="mf-page-head__sub">{sub}</div>}
      </div>
      {actions && <div className="mf-page-head__actions">{actions}</div>}
    </div>
  );
}
