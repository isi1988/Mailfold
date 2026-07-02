import React from 'react';
import { cx } from '../../lib/cx.js';
import { Icon } from '../atoms/Icon.jsx';

// Compact page window: first, last, current ± 1, with "…" gaps.
function pageWindow(page, total) {
  const keep = new Set([1, total, page, page - 1, page + 1]);
  const nums = [...keep].filter(n => n >= 1 && n <= total).sort((a, b) => a - b);
  const out = [];
  let prev = 0;
  for (const n of nums) {
    if (n - prev > 1) out.push('…');
    out.push(n);
    prev = n;
  }
  return out;
}

/**
 * Table / list pagination.
 *   page       current page (1-based)
 *   pageCount  total number of pages
 *   summary    optional left-aligned caption, e.g. "1–20 of 128"
 *   onPage     (n) => void
 */
export function Pagination({ page = 1, pageCount = 1, summary, onPage, className = '', ...rest }) {
  const go = n => { if (onPage && n >= 1 && n <= pageCount && n !== page) onPage(n); };
  return (
    <div className={cx('mf-pager', className)} {...rest}>
      {summary && <span className="mf-pager__summary">{summary}</span>}
      <div className="mf-pager__ctrls">
        <button className="mf-pager__nav" disabled={page <= 1} onClick={() => go(page - 1)} aria-label="Previous page">
          <Icon name="chevron-left" size={15} />
        </button>
        {pageWindow(page, pageCount).map((it, i) => it === '…'
          ? <span key={'gap' + i} className="mf-pager__gap">…</span>
          : <button key={it} className={cx('mf-pager__page', it === page && 'mf-pager__page--active')} aria-current={it === page ? 'page' : undefined} onClick={() => go(it)}>{it}</button>
        )}
        <button className="mf-pager__nav" disabled={page >= pageCount} onClick={() => go(page + 1)} aria-label="Next page">
          <Icon name="chevron-right" size={15} />
        </button>
      </div>
    </div>
  );
}
