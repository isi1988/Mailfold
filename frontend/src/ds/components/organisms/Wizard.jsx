import React from 'react';
import { cx } from '../../lib/cx.js';
import { Icon } from '../atoms/Icon.jsx';
import { Button } from '../atoms/Button.jsx';
import { Stepper } from '../molecules/Stepper.jsx';

/**
 * Create-flow wizard shell (mailbox / domain / alias / sync).
 * Put the current step's fields in `children`.
 *   title        header text
 *   steps        [{ n, label }]
 *   step         active step number
 *   createLabel  label for the final button
 *   onClose / onBack / onNext / onCreate
 */
export function Wizard({ title, steps = [], step = 1, createLabel = 'Create', onClose, onBack, onNext, onCreate, className = '', children, ...rest }) {
  const isLast = step === steps.length;
  return (
    <div className="mf-overlay mf-overlay--right" onClick={onClose}>
      <div className={cx('mf-drawer mf-drawer--wide', className)} onClick={e => e.stopPropagation()} {...rest}>
        <div className="mf-drawer__head">
          <div>
            <div className="mf-drawer__title">{title}</div>
            <div className="mf-u-faint" style={{ fontSize: 12, marginTop: 2 }}>Step {step} of {steps.length}</div>
          </div>
          <div className="mf-modal-close mf-spacer" onClick={onClose}><Icon name="close" size={18} /></div>
        </div>
        <Stepper steps={steps} current={step} />
        <div style={{ flex: 1, overflow: 'auto', padding: 22 }}>{children}</div>
        <div className="mf-drawer__foot">
          {step > 1 && <Button variant="secondary" onClick={onBack}>← Back</Button>}
          <div className="mf-spacer mf-row" style={{ gap: 10 }}>
            <Button variant="ghost" onClick={onClose}>Cancel</Button>
            {!isLast && <Button variant="primary" onClick={onNext}>Next →</Button>}
            {isLast && <Button variant="primary" onClick={onCreate}>{createLabel}</Button>}
          </div>
        </div>
      </div>
    </div>
  );
}
