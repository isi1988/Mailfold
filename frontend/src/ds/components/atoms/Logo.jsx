import React from 'react';
import { cx } from '../../lib/cx.js';

/**
 * Mailfold wordmark + fold mark.
 * props:
 *   size      'sm' | 'md'          — controls wordmark size (default 'md')
 *   wordmark  boolean              — show the "Mailfold" text (default true)
 *   markSize  number               — override the glyph px size
 *   color     css color            — glyph stroke/fill (default var(--ink))
 */
export function Logo({ size = 'md', wordmark = true, markSize, color = 'var(--ink)', className = '', ...rest }) {
  const s = markSize || (size === 'sm' ? 23 : 30);
  return (
    <div className={cx('mf-logo', 'mf-logo--' + size, className)} {...rest}>
      <svg width={s} height={s} viewBox="0 0 26 26" fill="none" aria-hidden="true">
        <rect x="3.4" y="3.4" width="19.2" height="19.2" rx="5.5" stroke={color} strokeWidth="1.7" />
        <path d="M15.4 3.5 H22.5 V10.6 Z" fill={color} />
      </svg>
      {wordmark && <span className="mf-logo__word">Mailfold</span>}
    </div>
  );
}
