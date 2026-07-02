import React from 'react';
import { cx } from '../../lib/cx.js';
import { Icon } from '../atoms/Icon.jsx';

/** A row in the webmail message list. */
export function MailListItem({ from, subject, preview, time, unread = false, starred = false, active = false, onClick, className = '', ...rest }) {
  return (
    <div className={cx('mf-mail-item', unread && 'mf-mail-item--unread', active && 'mf-mail-item--active', className)} onClick={onClick} {...rest}>
      <div className="mf-mail-item__rail">
        {unread && <span className="mf-dot mf-dot--accent" style={{ width: 8, height: 8 }} />}
        <Icon name="star" size={14} style={{ color: starred ? 'var(--amber)' : 'var(--faint)' }} />
      </div>
      <div className="mf-min0" style={{ flex: 1 }}>
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 8 }}>
          <span className="mf-mail-item__from mf-truncate">{from}</span>
          <span className="mf-mail-item__time">{time}</span>
        </div>
        <div className="mf-mail-item__subject mf-truncate">{subject}</div>
        <div className="mf-mail-item__preview mf-truncate">{preview}</div>
      </div>
    </div>
  );
}
