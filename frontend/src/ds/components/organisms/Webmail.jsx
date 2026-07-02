import React from 'react';
import { cx } from '../../lib/cx.js';
import { Icon } from '../atoms/Icon.jsx';
import { Avatar } from '../atoms/Avatar.jsx';
import { Button } from '../atoms/Button.jsx';
import { IconButton } from '../atoms/IconButton.jsx';
import { Checkbox } from '../atoms/Checkbox.jsx';
import { SearchInput } from '../molecules/SearchInput.jsx';
import { FolderItem } from '../molecules/FolderItem.jsx';
import { LabelItem } from '../molecules/LabelItem.jsx';
import { MailListItem } from '../molecules/MailListItem.jsx';

const SYS_ICON = { Inbox: 'inbox', Sent: 'send', Drafts: 'drafts', Archive: 'archive', Junk: 'shield', Trash: 'trash' };

/**
 * Webmail three-pane: folders · message list · reading pane.
 *   folders   [{name,count,active}]
 *   labels    [{name,color}]
 *   emails    [{from,subject,preview,time,unread,starred,fromAddr,body,initials}]
 *   selected  index of the open message
 *   account   {email}
 */
export function Webmail({ folders = [], labels = [], emails = [], selected = 0, account = {}, className = '', ...rest }) {
  const mail = emails[selected] || emails[0] || {};
  const acct = account.email || 'jamie@acme.io';
  return (
    <div className={cx('mf-webmail', className)} {...rest}>
      {/* Folder rail */}
      <div className="mf-webmail__folders">
        <div className="mf-side-label">Favourites</div>
        <FolderItem icon={<Icon name="star" size={14} style={{ color: 'var(--amber)' }} />} label="Starred" count={2} />
        <FolderItem icon={<Icon name="flag" size={14} style={{ color: 'var(--red)' }} />} label="Flagged" count={3} />
        <FolderItem icon={<Icon name="clock" size={14} style={{ color: 'var(--faint)' }} />} label="Snoozed" />

        <div className="mf-row" style={{ gap: 7, padding: '13px 10px 5px' }}>
          <Icon name="chevron-down" size={12} style={{ color: 'var(--faint)' }} />
          <span className="mf-u-mono mf-u-muted" style={{ fontSize: 11, fontWeight: 600 }}>{acct}</span>
        </div>
        {folders.map(f => (
          <FolderItem key={f.name} active={f.active} count={f.count}
            icon={<Icon name={SYS_ICON[f.name] || 'folder'} size={15} style={{ color: f.active ? 'var(--accent-ink)' : 'var(--faint)' }} />}
            label={f.name} />
        ))}

        {/* A couple of custom folder trees for texture */}
        <FolderItem icon={<><Icon name="chevron-down" size={11} style={{ color: 'var(--faint)' }} /><Icon name="folder" size={15} style={{ color: 'var(--accent-ink)' }} /></>} label="Work" />
        <FolderItem nested icon={<Icon name="folder" size={15} style={{ color: 'var(--faint)' }} />} label="Clients" count={4} />
        <FolderItem nested icon={<Icon name="folder" size={15} style={{ color: 'var(--faint)' }} />} label="Invoices" />
        <FolderItem icon={<><Icon name="chevron-right" size={11} style={{ color: 'var(--faint)' }} /><Icon name="folder" size={15} style={{ color: 'var(--faint)' }} /></>} label="Newsletters" count={12} />

        <div className="mf-side-label">Labels</div>
        {labels.map(l => <LabelItem key={l.name} color={l.color} label={l.name} />)}
        <div className="mf-row" style={{ gap: 9, padding: '7px 10px', font: '600 12.5px var(--font-sans)', color: 'var(--accent-ink)', cursor: 'pointer' }}>
          <span style={{ width: 15, textAlign: 'center' }}>+</span>New label
        </div>
      </div>

      {/* Message list */}
      <div className="mf-webmail__list">
        <div className="mf-webmail__list-head">
          <Checkbox />
          <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>Inbox</span>
          <span className="mf-u-faint" style={{ fontSize: 12 }}>7 unread</span>
          <SearchInput sm placeholder="Search mail…" className="mf-spacer" style={{ width: 150 }} />
        </div>
        {emails.map((e, i) => (
          <MailListItem key={i} {...e} active={i === selected} />
        ))}
      </div>

      {/* Reading pane */}
      <div className="mf-webmail__reader">
        <div className="mf-webmail__toolbar">
          <Button variant="primary" size="sm"><Icon name="reply" size={14} />Reply</Button>
          <Button variant="secondary" size="sm"><Icon name="forward" size={14} />Forward</Button>
          <div className="mf-spacer mf-row" style={{ gap: 2 }}>
            <IconButton><Icon name="archive" size={16} /></IconButton>
            <IconButton><Icon name="trash" size={16} /></IconButton>
            <IconButton><Icon name="star" size={16} style={{ color: mail.starred ? 'var(--amber)' : 'var(--faint)' }} /></IconButton>
          </div>
        </div>
        <div className="mf-webmail__body">
          <div style={{ fontFamily: 'var(--font-serif)', fontSize: 23, fontWeight: 600, color: 'var(--ink-strong)', lineHeight: 1.25 }}>{mail.subject}</div>
          <div className="mf-row" style={{ gap: 12, margin: '16px 0 22px' }}>
            <Avatar size={40}>{mail.initials}</Avatar>
            <div className="mf-min0" style={{ flex: 1 }}>
              <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--ink)' }}>{mail.from}</div>
              <div className="mf-u-faint mf-u-mono" style={{ fontSize: 12.5 }}>{mail.fromAddr}</div>
            </div>
            <div className="mf-u-faint" style={{ fontSize: 12.5 }}>{mail.time}</div>
          </div>
          <div style={{ font: '400 14px/1.7 var(--font-sans)', color: 'var(--ink)', whiteSpace: 'pre-line' }}>{mail.body}</div>
          <div className="mf-reply-box">Reply to {mail.from}…</div>
        </div>
      </div>
    </div>
  );
}
