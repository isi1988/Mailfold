import React, { useState } from 'react';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Textarea } from '../ds/components/atoms/Textarea.jsx';
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

const gotoList = s => s.split(/[\n,]/).map(x => x.trim()).filter(Boolean);

export function AliasDrawer({ mode, alias, domains = [], onClose, onSaved, onDelete }) {
  const t = useT();
  const { toast } = useToast();
  const editing = mode === 'edit';

  const [localPart, setLocalPart] = useState('');
  const [domain, setDomain] = useState(domains[0] ? domains[0].domain_name : '');
  const [goto, setGoto] = useState(editing ? (alias.goto || '').split(',').join('\n') : '');
  const [active, setActive] = useState(editing ? isActive(alias.active) : true);
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (busy) return;
    const recipients = gotoList(goto);
    if ((!editing && (!localPart.trim() || !domain)) || recipients.length === 0) {
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
        const address = localPart.trim() + '@' + domain;
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
      subtitle={editing ? alias.address : (localPart ? localPart + '@' + domain : '')}
      icon={<Icon name="arrow-right" size={20} style={{ color: 'var(--accent-ink)' }} />}
      footer={footer}
      onClose={onClose}
    >
      {!editing && (
        <div className="mf-row" style={{ gap: 10 }}>
          <FormField label={t('aliases.form.localPart')} style={{ flex: 1 }}>
            <Input placeholder="team" value={localPart} onChange={e => setLocalPart(e.target.value)} />
          </FormField>
          <FormField label={t('aliases.form.domain')} style={{ flex: 1 }}>
            <select className="mf-input" value={domain} onChange={e => setDomain(e.target.value)}>
              {domains.map(d => <option key={d.domain_name} value={d.domain_name}>{d.domain_name}</option>)}
            </select>
          </FormField>
        </div>
      )}
      <FormField label={t('aliases.form.goto')}>
        <Textarea placeholder="one@example.com&#10;two@example.com" value={goto} onChange={e => setGoto(e.target.value)} style={{ minHeight: 110 }} />
      </FormField>
      <div className="mf-u-faint" style={{ fontSize: 11.5, marginTop: -4, marginBottom: 8 }}>{t('aliases.form.gotoHint')}</div>
      <div className="mf-row mf-row--between" style={{ marginTop: 6 }}>
        <span className="mf-u-muted" style={{ fontSize: 13 }}>{t('aliases.form.active')}</span>
        <Toggle on={active} onClick={() => setActive(a => !a)} style={{ cursor: 'pointer' }} />
      </div>
    </Drawer>
  );
}
