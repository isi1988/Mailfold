import React from 'react';
import { cx } from '../../lib/cx.js';
import { Icon } from '../atoms/Icon.jsx';
import { Chip } from '../atoms/Chip.jsx';

/**
 * A row in the webmail message list.
 * assignedTo/notesCount are only ever set inside a shared/team mailbox (see
 * MessageHeader.AssignedTo/NotesCount on the backend) — an ordinary
 * mailbox's messages never carry them, so the badges simply don't render.
 */
export function MailListItem({ from, subject, preview, time, unread = false, starred = false, active = false, assignedTo = '', notesCount = 0, onClick, className = '', ...rest }) {
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
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 6 }}>
          <div className="mf-mail-item__preview mf-truncate" style={{ flex: 1 }}>{preview}</div>
          {(assignedTo || notesCount > 0) && (
            <div style={{ display: 'flex', gap: 4, flex: 'none' }}>
              {assignedTo && <Chip tone="accent" style={{ fontSize: 10.5, padding: '1px 6px' }}>{assignedTo}</Chip>}
              {notesCount > 0 && <Chip style={{ fontSize: 10.5, padding: '1px 6px' }}>{notesCount}</Chip>}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
