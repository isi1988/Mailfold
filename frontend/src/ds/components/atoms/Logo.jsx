import React from 'react';
import { cx } from '../../lib/cx.js';

// The enterprise build sets VITE_BRAND=enterprise to swap the fold glyph for the
// bull mark. The mark is loaded from a runtime public path (never an import), so
// the open-source bundle stays free of the enterprise asset — the file simply
// does not exist in the open-source image and the branch is never taken.
const ENTERPRISE_BRAND = import.meta.env.VITE_BRAND === 'enterprise';

/**
 * Mailfold wordmark + mark.
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
      {ENTERPRISE_BRAND ? (
        <img src="/brand/enterprise-mark.png" width={s} height={s} alt="" aria-hidden="true" style={{ display: 'block', objectFit: 'contain' }} />
      ) : (
        <svg width={s} height={s} viewBox="0 0 26 26" fill="none" aria-hidden="true">
          <rect x="3.4" y="3.4" width="19.2" height="19.2" rx="5.5" stroke={color} strokeWidth="1.7" />
          <path d="M15.4 3.5 H22.5 V10.6 Z" fill={color} />
        </svg>
      )}
      {wordmark && <span className="mf-logo__word">Mailfold</span>}
    </div>
  );
}
