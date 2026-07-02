import React from 'react';
import { cx } from '../../lib/cx.js';
import { Icon } from '../atoms/Icon.jsx';
import { Button } from '../atoms/Button.jsx';
import { Textarea } from '../atoms/Textarea.jsx';
import { MultiSelect } from '../molecules/MultiSelect.jsx';

/**
 * Compose slide-over.
 *   options    [string] recipient suggestions
 *   to/cc/bcc  [string] preselected recipients
 *   ccShown    boolean — reveal Cc & Bcc rows
 *   onClose
 */
export function ComposeModal({ options = [], to = [], cc = [], bcc = [], ccShown = false, onClose, className = '', ...rest }) {
  const rows = [{ label: 'To', values: to, ph: 'name@example.com', toggle: true }];
  if (ccShown) {
    rows.push({ label: 'Cc', values: cc, ph: 'Add Cc recipients' });
    rows.push({ label: 'Bcc', values: bcc, ph: 'Add Bcc recipients' });
  }
  return (
    <div className="mf-overlay mf-overlay--right" onClick={onClose}>
      <div className={cx('mf-drawer', className)} onClick={e => e.stopPropagation()} {...rest}>
        <div className="mf-drawer__head">
          <div className="mf-drawer__title">New message</div>
          <div className="mf-modal-close mf-spacer" onClick={onClose}><Icon name="close" size={18} /></div>
        </div>
        <div style={{ padding: '16px 20px', display: 'flex', flexDirection: 'column', gap: 12, overflow: 'auto', flex: 1 }}>
          {rows.map(r => (
            <div key={r.label} style={{ display: 'flex', alignItems: 'flex-start', gap: 10, borderBottom: '1px solid var(--hair-soft)', paddingBottom: 9 }}>
              <span style={{ fontSize: 12.5, color: 'var(--faint)', width: 32, flex: 'none', paddingTop: 8 }}>{r.label}</span>
              <div style={{ flex: 1, minWidth: 0 }}><MultiSelect flat values={r.values} options={options} placeholder={r.ph} /></div>
              {r.toggle && <span style={{ fontSize: 12, color: 'var(--faint)', cursor: 'pointer', flex: 'none', paddingTop: 8 }}>Cc · Bcc</span>}
            </div>
          ))}
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, borderBottom: '1px solid var(--hair-soft)', paddingBottom: 11 }}>
            <span style={{ fontSize: 12.5, color: 'var(--faint)', width: 56 }}>Subject</span>
            <input placeholder="Subject" style={{ flex: 1, border: 'none', background: 'transparent', outline: 'none', fontSize: 13.5, fontWeight: 500, color: 'var(--ink)' }} />
          </div>
          <Textarea placeholder="Write your message…" />
        </div>
        <div className="mf-drawer__foot">
          <Button variant="primary">Send</Button>
          <span className="mf-u-faint" style={{ fontSize: 12 }}>Draft saved</span>
          <Button variant="link" className="mf-spacer">Discard</Button>
        </div>
      </div>
    </div>
  );
}
