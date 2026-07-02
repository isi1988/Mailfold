import React from 'react';
import { cx } from '../../lib/cx.js';

/**
 * Wizard step indicator.
 *   steps    [{ n, label }]
 *   current  active step number
 */
export function Stepper({ steps = [], current = 1, className = '', ...rest }) {
  return (
    <div className={cx('mf-stepper', className)} {...rest}>
      {steps.map((s, i) => {
        const done = s.n < current;
        const active = s.n === current;
        return (
          <React.Fragment key={s.n}>
            <div className="mf-stepper__step">
              <div className={cx('mf-stepper__num', active && 'mf-stepper__num--active', done && 'mf-stepper__num--done')}>{s.n}</div>
              <span className={cx('mf-stepper__label', (active || done) && 'mf-stepper__label--on')}>{s.label}</span>
            </div>
            {i < steps.length - 1 && <div className={cx('mf-stepper__line', done && 'mf-stepper__line--done')} />}
          </React.Fragment>
        );
      })}
    </div>
  );
}
