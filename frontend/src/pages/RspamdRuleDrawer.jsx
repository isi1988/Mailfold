import React, { useState } from 'react';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Textarea } from '../ds/components/atoms/Textarea.jsx';
import { Toggle } from '../ds/components/atoms/Toggle.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { api } from '../api/client.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';

/**
 * Create a custom Rspamd setting (a raw rule block that overrides spam
 * scoring/whitelisting for matched messages). mailcow has no edit verb for
 * these, so this drawer only ever creates.
 *   onClose  () => void
 *   onSaved  () => void
 */
export function RspamdRuleDrawer({ onClose, onSaved }) {
  const t = useT();
  const { toast } = useToast();
  const [desc, setDesc] = useState('');
  const [content, setContent] = useState('');
  const [active, setActive] = useState(true);
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (busy || !content.trim()) {
      toast(t('spam.rules.form.invalid'));
      return;
    }
    setBusy(true);
    try {
      await api.post('/api/rspamd-settings', { desc: desc.trim(), content, active: active ? '1' : '0' });
      toast(t('spam.rules.form.created'));
      onSaved();
      onClose();
    } catch (err) {
      toast(t('spam.rules.form.failed'), (err && err.message) || '');
    } finally {
      setBusy(false);
    }
  }

  const footer = (
    <>
      <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
      <Button variant="primary" onClick={submit} disabled={busy}>
        {busy ? t('spam.rules.form.creating') : t('spam.rules.form.create')}
      </Button>
    </>
  );
  return (
    <Drawer title={t('spam.rules.form.newTitle')} subtitle={t('spam.rules.sub')} footer={footer} onClose={onClose}>
      <FormField label={t('spam.rules.form.desc')}>
        <Input placeholder={t('spam.rules.form.descPlaceholder')} value={desc} onChange={e => setDesc(e.target.value)} />
      </FormField>
      <FormField label={t('spam.rules.form.content')}>
        <Textarea
          mono
          rows={10}
          placeholder={'priority = 5;\nrcpt = "user@example.com";\napply "SUBJECT_TAG" = 1;'}
          value={content}
          onChange={e => setContent(e.target.value)}
        />
        <div className="mf-u-faint" style={{ fontSize: 11, marginTop: 5 }}>{t('spam.rules.form.contentHint')}</div>
      </FormField>
      <div className="mf-row mf-row--between" style={{ marginTop: 4 }}>
        <span style={{ fontSize: 13.5, color: 'var(--ink)' }}>{t('spam.rules.form.active')}</span>
        <Toggle on={active} onClick={() => setActive(a => !a)} style={{ cursor: 'pointer' }} />
      </div>
    </Drawer>
  );
}
