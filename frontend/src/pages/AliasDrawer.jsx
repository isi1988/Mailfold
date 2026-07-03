import React, { useState } from 'react';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Token } from '../ds/components/atoms/Token.jsx';
import { Avatar } from '../ds/components/atoms/Avatar.jsx';
import { Toggle } from '../ds/components/atoms/Toggle.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { api } from '../api/client.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';
import { isActive } from '../lib/format.js';

function errText(err, fallback) {
  return (err && err.body && err.body.message) || (err && err.message) || fallback;
}

// 2-letter initials from an email local-part, e.g. "a.ruiz@x" -> "AR"
function addrInitials(addr) {
  const name = String(addr).split('@')[0];
  const parts = name.split(/[._+-]+/).filter(Boolean);
  const ini = parts.length >= 2 ? parts[0][0] + parts[1][0] : name.slice(0, 2);
  return ini.toUpperCase();
}

// RecipientPicker is a chip field for the alias's forwarding targets: type an
// address and press Enter/comma/space to add it, or pick one of the existing
// mailboxes from the suggestion menu that opens on focus.
function RecipientPicker({ values, onChange, options = [] }) {
  const [text, setText] = useState('');
  const [open, setOpen] = useState(false);

  function commit(raw) {
    const parts = raw.split(/[,\s]+/).map(s => s.trim()).filter(Boolean);
    if (parts.length) onChange(Array.from(new Set([...values, ...parts])));
  }
  function pick(addr) {
    onChange(Array.from(new Set([...values, addr])));
    setText('');
  }

  const remaining = options.filter(o => !values.includes(o) && (!text || o.toLowerCase().includes(text.toLowerCase())));

  return (
    <div className={'mf-multiselect' + (open ? ' mf-multiselect--open' : '')} onBlur={e => { if (!e.currentTarget.contains(e.relatedTarget)) setOpen(false); }}>
      <div className="mf-multiselect__box" onClick={() => setOpen(true)}>
        {values.map(v => <Token key={v} label={v} onRemove={() => onChange(values.filter(x => x !== v))} />)}
        <input
          className="mf-multiselect__input"
          placeholder={values.length ? '' : 'jamie@acme.io'}
          value={text}
          onFocus={() => setOpen(true)}
          onChange={e => setText(e.target.value)}
          onKeyDown={e => {
            if ((e.key === 'Enter' || e.key === ',' || e.key === ' ') && text.trim()) { e.preventDefault(); commit(text); setText(''); }
            else if (e.key === 'Backspace' && !text && values.length) onChange(values.slice(0, -1));
          }}
        />
      </div>
      {open && remaining.length > 0 && (
        <div className="mf-select__menu">
          {remaining.slice(0, 8).map(o => (
            <div key={o} className="mf-select__opt" onMouseDown={e => { e.preventDefault(); pick(o); }}>
              <Avatar size={24}>{addrInitials(o)}</Avatar>
              <span className="mf-truncate" style={{ flex: 1, fontFamily: 'var(--font-mono)' }}>{o}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export function AliasDrawer({ mode, alias, domains = [], mailboxes = [], onClose, onSaved, onDelete }) {
  const t = useT();
  const { toast } = useToast();
  const editing = mode === 'edit';
  const editingCatchAll = editing && alias.address.startsWith('@');

  const [localPart, setLocalPart] = useState('');
  const [domain, setDomain] = useState(domains[0] ? domains[0].domain_name : '');
  const [catchAll, setCatchAll] = useState(false);
  const [recipients, setRecipients] = useState(editing ? (alias.goto || '').split(',').map(s => s.trim()).filter(Boolean) : []);
  const [active, setActive] = useState(editing ? isActive(alias.active) : true);
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (busy) return;
    if ((!editing && (!catchAll && !localPart.trim())) || (!editing && !domain) || recipients.length === 0) {
      toast(t('aliases.form.invalid'));
      return;
    }
    setBusy(true);
    try {
      if (editing) {
        await api.put('/api/aliases', {
          items: [String(alias.id)],
          attr: { address: alias.address, goto: recipients.join(','), active: active ? '1' : '0' },
        });
        toast(t('aliases.form.updated', { alias: alias.address }));
      } else {
        const address = catchAll ? '@' + domain : localPart.trim() + '@' + domain;
        await api.post('/api/aliases', { address, goto: recipients.join(','), active: active ? '1' : '0' });
        toast(t('aliases.form.created', { alias: address }));
      }
      onSaved();
      onClose();
    } catch (err) {
      toast(t('aliases.form.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  const footer = (
    <>
      {editing && onDelete && <Button variant="danger" onClick={() => onDelete(alias)}>{t('common.delete')}</Button>}
      <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
      <Button variant="primary" onClick={submit} disabled={busy}>
        {busy ? t('aliases.form.saving') : (editing ? t('common.save') : t('aliases.form.create'))}
      </Button>
    </>
  );

  return (
    <Drawer
      title={editing ? t('aliases.form.editTitle') : t('aliases.form.newTitle')}
      subtitle={editing ? alias.address : (catchAll ? '@' + domain : (localPart ? localPart + '@' + domain : ''))}
      icon={<Icon name="arrow-right" size={20} style={{ color: 'var(--accent-ink)' }} />}
      footer={footer}
      onClose={onClose}
    >
      {!editing && (
        <>
          <div className="mf-row" style={{ gap: 10 }}>
            <FormField label={t('aliases.form.localPart')} style={{ flex: 1 }}>
              <Input placeholder="team" value={localPart} onChange={e => setLocalPart(e.target.value)} disabled={catchAll} />
            </FormField>
            <FormField label={t('aliases.form.domain')} style={{ flex: 1 }}>
              <select className="mf-input" value={domain} onChange={e => setDomain(e.target.value)}>
                {domains.map(d => <option key={d.domain_name} value={d.domain_name}>{d.domain_name}</option>)}
              </select>
            </FormField>
          </div>
          <div className="mf-row mf-row--between" style={{ margin: '10px 0 18px' }}>
            <span>
              <span className="mf-u-muted" style={{ fontSize: 13 }}>{t('aliases.form.catchAll')}</span>
              <div className="mf-u-faint" style={{ fontSize: 11.5, marginTop: 2 }}>{t('aliases.form.catchAllHint')}</div>
            </span>
            <Toggle on={catchAll} onClick={() => setCatchAll(v => !v)} style={{ cursor: 'pointer', flex: 'none' }} />
          </div>
        </>
      )}
      {editingCatchAll && (
        <div className="mf-u-faint" style={{ fontSize: 12, marginBottom: 10 }}>{t('aliases.form.catchAllHint')}</div>
      )}
      <FormField label={t('aliases.form.goto')}>
        <RecipientPicker values={recipients} onChange={setRecipients} options={mailboxes} />
      </FormField>
      <div className="mf-u-faint" style={{ fontSize: 11.5, marginTop: 6, marginBottom: 8 }}>{t('aliases.form.gotoHint')}</div>
      <div className="mf-row mf-row--between" style={{ marginTop: 6 }}>
        <span className="mf-u-muted" style={{ fontSize: 13 }}>{t('aliases.form.active')}</span>
        <Toggle on={active} onClick={() => setActive(a => !a)} style={{ cursor: 'pointer' }} />
      </div>
    </Drawer>
  );
}
