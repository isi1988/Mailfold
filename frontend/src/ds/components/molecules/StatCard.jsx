import React from 'react';
import { cx } from '../../lib/cx.js';

/**
 * KPI / statistic card.
 *   label, value              text
 *   valueTone                 'green' | 'amber' | 'red' | 'blue' — colours the number
 *   delta                     small line under the number (node)
 *   deltaTone                 'green' | 'amber' | 'red' | 'muted'
 *   dot                       boolean — status dot before the delta
 *   pct, note                 render a progress bar + caption instead of a delta
 *   icon                      node — floated top-right
 *   size                      'md' | 'sm'
 */
export function StatCard({ label, value, valueTone, delta, deltaTone = 'muted', dot = false, pct, note, icon, size = 'md', className = '', ...rest }) {
  return (
    <div className={cx('mf-stat', size === 'lg' && 'mf-stat--lg', size === 'sm' && 'mf-stat--sm', className)} {...rest}>
      {icon && <div className="mf-stat__icon">{icon}</div>}
      <div className="mf-stat__label">{label}</div>
      <div className="mf-stat__value" style={valueTone ? { color: 'var(--' + valueTone + ')' } : undefined}>{value}</div>
      {typeof pct === 'number' && (
        <>
          <div className="mf-bar" style={{ marginTop: 11 }}><span className="mf-bar__fill" style={{ width: pct + '%' }} /></div>
          {note && <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 7 }}>{note}</div>}
        </>
      )}
      {delta && (
        <div className={cx('mf-stat__delta', 'mf-u-' + deltaTone)}>
          {dot && <span className={cx('mf-dot', deltaTone !== 'green' && 'mf-dot--' + deltaTone)} />}
          {delta}
        </div>
      )}
    </div>
  );
}
