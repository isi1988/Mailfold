import React from 'react';
import { cx } from '../../lib/cx.js';
import { Icon } from '../atoms/Icon.jsx';

/**
 * Right-hand slide-over panel.
 *   title, subtitle   header text
 *   icon              node shown left of the title (e.g. an Avatar)
 *   footer            node pinned to the bottom bar
 *   wide              boolean — 520px instead of 468px
 *   overlay           boolean — wrap in a dimmed backdrop (default true)
 *   onClose           () => void
 */
export function Drawer({ title, subtitle, icon, footer, wide = false, overlay = true, onClose, className = '', children, ...rest }) {
  const panel = (
    <div className={cx('mf-drawer', wide && 'mf-drawer--wide', className)} onClick={e => e.stopPropagation()} {...rest}>
      <div className="mf-drawer__head">
        {icon}
        <div className="mf-min0" style={{ flex: 1 }}>
          <div className="mf-drawer__title">{title}</div>
          {subtitle && <div className="mf-u-faint mf-u-mono mf-truncate" style={{ fontSize: 12.5 }}>{subtitle}</div>}
        </div>
        <div className="mf-modal-close" onClick={onClose}><Icon name="close" size={18} /></div>
      </div>
      <div className="mf-drawer__body">{children}</div>
      {footer && <div className="mf-drawer__foot">{footer}</div>}
    </div>
  );
  if (!overlay) return panel;
  return <div className="mf-overlay mf-overlay--right" onClick={onClose}>{panel}</div>;
}
