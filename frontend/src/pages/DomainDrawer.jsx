import React, { useState, useEffect, useCallback } from 'react';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Toggle } from '../ds/components/atoms/Toggle.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Logo } from '../ds/components/atoms/Logo.jsx';
import { Divider } from '../ds/components/atoms/Divider.jsx';
import { api } from '../api/client.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';
import { isActive } from '../lib/format.js';
import { decodeIdnDomain } from '../lib/idn.js';

// The list exposes the per-mailbox max quota in bytes; the write API takes MB.
const bytesToGB = b => Math.max(1, Math.round((Number(b) || 0) / 1024 / 1024 / 1024)) || 1;

function errText(err, fallback) {
  return (err && err.body && err.body.message) || (err && err.message) || fallback;
}

// mailcow returns the DKIM record as an object; an absent key comes back as an
// empty array / empty object, so a present record is one that carries a TXT value.
function hasDkim(rec) {
  return Boolean(rec && !Array.isArray(rec) && (rec.dkim_txt || rec.pubkey));
}

// The rate-limit passthrough is keyed by domain: { "example.com": { value, frame } }.
function pickRateLimit(payload, domainName) {
  if (!payload || Array.isArray(payload)) return null;
  const entry = payload[domainName] || payload;
  if (!entry || typeof entry !== 'object') return null;
  return { value: entry.value != null ? String(entry.value) : '', frame: entry.frame || 's' };
}

/** DKIM key management for an existing domain (edit mode only). */
function DkimSection({ domainName }) {
  const t = useT();
  const { toast } = useToast();
  const [rec, setRec] = useState(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [keySize, setKeySize] = useState('2048');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setRec(await api.get('/api/dkim/' + encodeURIComponent(domainName)));
    } catch {
      setRec(null);
    } finally {
      setLoading(false);
    }
  }, [domainName]);

  useEffect(() => { load(); }, [load]);

  async function generate() {
    if (busy) return;
    setBusy(true);
    try {
      await api.post('/api/dkim', { domains: domainName, dkim_selector: 'dkim', key_size: keySize });
      toast(t('domains.dkim.generated', { domain: decodeIdnDomain(domainName) }));
      await load();
    } catch (err) {
      toast(t('domains.dkim.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (busy) return;
    setBusy(true);
    try {
      await api.del('/api/dkim', { items: [domainName] });
      toast(t('domains.dkim.deleted', { domain: decodeIdnDomain(domainName) }));
      await load();
    } catch (err) {
      toast(t('domains.dkim.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  function copyTxt() {
    const txt = rec && rec.dkim_txt;
    if (!txt) return;
    try {
      navigator.clipboard.writeText(txt);
      toast(t('domains.dkim.copied'));
    } catch {
      /* clipboard unavailable — no-op */
    }
  }

  const present = hasDkim(rec);

  return (
    <div style={{ marginTop: 4 }}>
      <div className="mf-row mf-row--between" style={{ marginBottom: 8 }}>
        <span className="mf-label mf-label--strong">{t('domains.dkim.title')}</span>
        {present && (
          <span className="mf-u-faint" style={{ fontSize: 11.5 }}>
            {t('domains.dkim.keyInfo', { selector: rec.dkim_selector || 'dkim', length: rec.length || keySize })}
          </span>
        )}
      </div>

      {loading ? (
        <div className="mf-u-muted" style={{ fontSize: 13 }}>{t('common.loading')}</div>
      ) : present ? (
        <>
          <FormField label={t('domains.dkim.txtRecord')}>
            <textarea
              className="mf-input mf-u-mono"
              readOnly
              rows={4}
              value={rec.dkim_txt || ''}
              style={{ resize: 'vertical', fontSize: 11.5, wordBreak: 'break-all' }}
            />
          </FormField>
          <div className="mf-row" style={{ gap: 8, marginTop: 6 }}>
            <Button variant="secondary" size="sm" onClick={copyTxt}>{t('domains.dkim.copy')}</Button>
            <Button variant="danger" size="sm" className="mf-spacer" onClick={remove} disabled={busy}>
              {t('domains.dkim.rotate')}
            </Button>
          </div>
        </>
      ) : (
        <>
          <div className="mf-u-muted" style={{ fontSize: 13, marginBottom: 8 }}>{t('domains.dkim.none')}</div>
          <div className="mf-row" style={{ gap: 8, alignItems: 'flex-end' }}>
            <FormField label={t('domains.dkim.keySize')} style={{ width: 120 }}>
              <select className="mf-input" value={keySize} onChange={e => setKeySize(e.target.value)}>
                <option value="1024">1024</option>
                <option value="2048">2048</option>
              </select>
            </FormField>
            <Button variant="primary" size="sm" onClick={generate} disabled={busy}>
              {busy ? t('domains.dkim.generating') : t('domains.dkim.generate')}
            </Button>
          </div>
        </>
      )}
    </div>
  );
}

/** Per-domain outbound send rate limit (edit mode only). */
function RateLimitSection({ domainName }) {
  const t = useT();
  const { toast } = useToast();
  const [value, setValue] = useState('');
  const [frame, setFrame] = useState('s');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const rl = pickRateLimit(await api.get('/api/ratelimits/domain'), domainName);
      setValue(rl ? rl.value : '');
      setFrame(rl && rl.frame ? rl.frame : 's');
    } catch {
      setValue('');
      setFrame('s');
    } finally {
      setLoading(false);
    }
  }, [domainName]);

  useEffect(() => { load(); }, [load]);

  async function save() {
    if (busy) return;
    setBusy(true);
    try {
      await api.put('/api/ratelimits/domain', {
        items: [domainName],
        attr: { rl_value: value.trim(), rl_frame: frame },
      });
      toast(t('domains.rl.saved', { domain: decodeIdnDomain(domainName) }));
      await load();
    } catch (err) {
      toast(t('domains.rl.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div style={{ marginTop: 4 }}>
      <div className="mf-row mf-row--between" style={{ marginBottom: 8 }}>
        <span className="mf-label mf-label--strong">{t('domains.rl.title')}</span>
      </div>
      {loading ? (
        <div className="mf-u-muted" style={{ fontSize: 13 }}>{t('common.loading')}</div>
      ) : (
        <>
          <div className="mf-u-muted" style={{ fontSize: 12, marginBottom: 8 }}>{t('domains.rl.hint')}</div>
          <div className="mf-row" style={{ gap: 8, alignItems: 'flex-end' }}>
            <FormField label={t('domains.rl.messages')} style={{ flex: 1 }}>
              <Input
                type="number"
                min="0"
                align="right"
                placeholder={t('domains.rl.unlimited')}
                value={value}
                onChange={e => setValue(e.target.value)}
              />
            </FormField>
            <FormField label={t('domains.rl.per')} style={{ width: 140 }}>
              <select className="mf-input" value={frame} onChange={e => setFrame(e.target.value)}>
                <option value="s">{t('domains.rl.second')}</option>
                <option value="m">{t('domains.rl.minute')}</option>
                <option value="h">{t('domains.rl.hour')}</option>
                <option value="d">{t('domains.rl.day')}</option>
              </select>
            </FormField>
            <Button variant="primary" size="sm" onClick={save} disabled={busy}>
              {busy ? t('domains.rl.saving') : t('common.save')}
            </Button>
          </div>
        </>
      )}
    </div>
  );
}

export function DomainDrawer({ mode, domain, onClose, onSaved, onDelete }) {
  const t = useT();
  const { toast } = useToast();
  const editing = mode === 'edit';

  const [name, setName] = useState('');
  const [description, setDescription] = useState(editing ? domain.description || '' : '');
  const [quotaGB, setQuotaGB] = useState(editing ? bytesToGB(domain.max_quota_for_domain) : 10);
  const [maxMailboxes, setMaxMailboxes] = useState(10);
  const [active, setActive] = useState(editing ? isActive(domain.active) : true);
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (busy) return;
    if (!editing && !/^[^\s/@]+\.[^\s/@]+$/.test(name.trim())) {
      toast(t('domains.form.invalid'));
      return;
    }
    setBusy(true);
    const quotaMB = quotaGB * 1024;
    try {
      if (editing) {
        await api.put('/api/domains', {
          items: [domain.domain_name],
          attr: { description, maxquota: String(quotaMB), quota: String(quotaMB), active: active ? '1' : '0' },
        });
        toast(t('domains.form.updated', { domain: decodeIdnDomain(domain.domain_name) }));
      } else {
        await api.post('/api/domains', {
          domain: name.trim(),
          description,
          aliases: '400',
          mailboxes: String(maxMailboxes),
          defquota: String(Math.min(3072, quotaMB)),
          maxquota: String(quotaMB),
          quota: String(quotaMB),
          active: active ? '1' : '0',
        });
        toast(t('domains.form.created', { domain: name.trim() }));
      }
      onSaved();
      onClose();
    } catch (err) {
      toast(t('domains.form.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  const footer = (
    <>
      {editing && onDelete && <Button variant="danger" onClick={() => onDelete(domain)}>{t('common.delete')}</Button>}
      <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
      <Button variant="primary" onClick={submit} disabled={busy}>
        {busy ? t('domains.form.saving') : (editing ? t('common.save') : t('domains.form.create'))}
      </Button>
    </>
  );

  return (
    <Drawer
      title={editing ? t('domains.form.editTitle') : t('domains.form.newTitle')}
      subtitle={editing ? decodeIdnDomain(domain.domain_name) : name}
      icon={<div className="mf-avatar mf-avatar--square mf-avatar--34"><Logo wordmark={false} markSize={18} color="var(--accent-ink)" /></div>}
      footer={footer}
      onClose={onClose}
    >
      {!editing && (
        <FormField label={t('domains.form.domain')}>
          <Input mono placeholder="example.com" value={name} onChange={e => setName(e.target.value)} />
        </FormField>
      )}
      <FormField label={t('domains.form.description')}>
        <Input placeholder="Acme Inc." value={description} onChange={e => setDescription(e.target.value)} />
      </FormField>
      <FormField label={t('domains.form.quota')}>
        <Input type="number" min="1" align="right" value={quotaGB} onChange={e => setQuotaGB(Number(e.target.value) || 1)} />
      </FormField>
      {!editing && (
        <FormField label={t('domains.form.maxMailboxes')}>
          <Input type="number" min="1" align="right" value={maxMailboxes} onChange={e => setMaxMailboxes(Number(e.target.value) || 1)} />
        </FormField>
      )}
      <div className="mf-row mf-row--between" style={{ marginTop: 6 }}>
        <span className="mf-u-muted" style={{ fontSize: 13 }}>{t('domains.form.active')}</span>
        <Toggle on={active} onClick={() => setActive(a => !a)} style={{ cursor: 'pointer' }} />
      </div>

      {editing && (
        <>
          <Divider soft style={{ margin: '18px 0' }} />
          <DkimSection domainName={domain.domain_name} />
          <Divider soft style={{ margin: '18px 0' }} />
          <RateLimitSection domainName={domain.domain_name} />
        </>
      )}
    </Drawer>
  );
}
