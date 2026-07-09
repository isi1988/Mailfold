import React, { useState } from 'react';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { ConfirmModal } from '../ds/components/organisms/ConfirmModal.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Toggle } from '../ds/components/atoms/Toggle.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Avatar } from '../ds/components/atoms/Avatar.jsx';
import { initials } from '../ds/data/sample.js';
import { api } from '../api/client.js';
import { useApi } from '../lib/useApi.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';
import { isActive, asList } from '../lib/format.js';
import { decodeIdnAddress, decodeIdnDomain } from '../lib/idn.js';

// mailcow's GET returns the mailbox quota in bytes; the write path takes
// megabytes. Show whole gigabytes, at least 1.
const bytesToGB = b => Math.max(1, Math.round((Number(b) || 0) / 1024 / 1024 / 1024));

function randomPassword() {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789!@#$%';
  let out = '';
  const buf = new Uint32Array(16);
  crypto.getRandomValues(buf);
  for (let i = 0; i < buf.length; i += 1) out += chars[buf[i] % chars.length];
  return out;
}

function errText(err, fallback) {
  return (err && err.body && err.body.message) || (err && err.message) || fallback;
}

// A titled block that groups a sub-resource list inside the edit drawer.
function Section({ title, hint, children }) {
  return (
    <div style={{ marginTop: 22, paddingTop: 18, borderTop: '1px solid var(--hair)' }}>
      <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>{title}</div>
      {hint ? <div className="mf-u-faint" style={{ fontSize: 11.5, marginTop: 3 }}>{hint}</div> : null}
      <div style={{ marginTop: 12 }}>{children}</div>
    </div>
  );
}

// One line in a sub-resource list, with an optional remove button on the right.
function SectionRow({ children, onRemove, removeLabel, busy }) {
  return (
    <div
      className="mf-row mf-row--between"
      style={{ gap: 8, padding: '7px 0', borderBottom: '1px solid var(--hair)' }}
    >
      <div className="mf-min0" style={{ flex: 1 }}>{children}</div>
      {onRemove && (
        <Button variant="ghost" size="sm" onClick={onRemove} disabled={busy}>{removeLabel}</Button>
      )}
    </div>
  );
}

// mailcow returns rate-limit collections either as an array or as an object
// keyed by mailbox address; normalise to a flat array of entries.
function normalizeRows(data) {
  if (Array.isArray(data)) return data;
  if (data && typeof data === 'object') return Object.values(data);
  return [];
}

// The protocols an app password can be scoped to. mailcow's API expects
// these as an array of the enabled keys (NOT an object of booleans — sending
// an object silently creates a password with every protocol OFF, which is
// exactly the bug this list's shape exists to prevent regressing to).
const APP_PW_PROTOCOLS = ['imap_access', 'smtp_access', 'dav_access', 'eas_access', 'pop3_access', 'sieve_access'];

function ProtocolPicker({ selected, onToggle }) {
  const t = useT();
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px 14px', marginTop: 10 }}>
      {APP_PW_PROTOCOLS.map(p => (
        <label key={p} className="mf-row" style={{ gap: 6, fontSize: 12.5, color: 'var(--muted)', cursor: 'pointer' }}>
          <input type="checkbox" checked={selected.includes(p)} onChange={() => onToggle(p)} />
          {t('mailboxes.appPw.protocol.' + p)}
        </label>
      ))}
    </div>
  );
}

/**
 * App passwords — per-mailbox credentials for IMAP/SMTP clients that can be
 * revoked without touching the primary password. Edit-mode only.
 *
 * mailcow's `edit/app-passwd` API accepts requests without error but never
 * actually applies them — the admin UI itself only ever edits through a
 * separate HTML form (edit.php), never this JSON endpoint, so it's
 * effectively dead/untested on mailcow's side. "Editing" here is therefore
 * implemented as create-the-replacement-first, then delete-the-original —
 * safer than mailcow's own semantics would be if they worked, since the
 * mailbox is never left without a matching credential mid-operation.
 */
function AppPasswordsSection({ mailbox }) {
  const t = useT();
  const { toast } = useToast();
  const { data, loading, reload } = useApi('/api/app-passwords/' + encodeURIComponent(mailbox), [mailbox]);
  const [name, setName] = useState('');
  const [pw, setPw] = useState('');
  const [protocols, setProtocols] = useState(APP_PW_PROTOCOLS);
  const [busy, setBusy] = useState(false);
  const [editingId, setEditingId] = useState(null); // id being replaced, or null when adding fresh
  const [confirmDelete, setConfirmDelete] = useState(null);
  const rows = asList(data);

  const toggleProtocol = p => setProtocols(cur => (cur.includes(p) ? cur.filter(x => x !== p) : [...cur, p]));

  function resetForm() {
    setEditingId(null);
    setName('');
    setPw('');
    setProtocols(APP_PW_PROTOCOLS);
  }

  function startEdit(row) {
    setEditingId(row.id);
    setName(row.name || '');
    setPw('');
    setProtocols(APP_PW_PROTOCOLS.filter(p => row[p]));
  }

  async function submit() {
    if (busy) return;
    if (!name.trim() || pw.length < 8) { toast(t('mailboxes.appPw.invalid')); return; }
    if (protocols.length === 0) { toast(t('mailboxes.appPw.noProtocols')); return; }
    setBusy(true);
    try {
      await api.post('/api/app-passwords', {
        username: mailbox,
        app_name: name.trim(),
        app_passwd: pw,
        app_passwd2: pw,
        active: '1',
        protocols,
      });
      if (editingId != null) await api.del('/api/app-passwords', { items: [String(editingId)] });
      toast(editingId != null ? t('mailboxes.appPw.updated') : t('mailboxes.appPw.added'));
      resetForm();
      reload();
    } catch (err) {
      toast(t('mailboxes.appPw.failed'), errText(err, ''));
    } finally { setBusy(false); }
  }

  async function doDelete() {
    const row = confirmDelete;
    setConfirmDelete(null);
    setBusy(true);
    try {
      await api.del('/api/app-passwords', { items: [String(row.id)] });
      toast(t('mailboxes.appPw.removed'));
      if (editingId === row.id) resetForm();
      reload();
    } catch (err) {
      toast(t('mailboxes.appPw.failed'), errText(err, ''));
    } finally { setBusy(false); }
  }

  return (
    <Section title={t('mailboxes.appPw.title')} hint={t('mailboxes.appPw.hint')}>
      {loading ? (
        <div className="mf-u-faint" style={{ fontSize: 12.5 }}>{t('common.loading')}</div>
      ) : rows.length === 0 ? (
        <div className="mf-u-faint" style={{ fontSize: 12.5 }}>{t('mailboxes.appPw.empty')}</div>
      ) : (
        rows.map(r => (
          <div key={r.id} className="mf-row mf-row--between" style={{ gap: 8, padding: '7px 0', borderBottom: '1px solid var(--hair)' }}>
            <div className="mf-min0" style={{ flex: 1 }}>
              <div style={{ fontSize: 13, color: 'var(--ink)' }}>{r.name || t('mailboxes.appPw.unnamed')}</div>
              <div className="mf-u-faint" style={{ fontSize: 11, marginTop: 2 }}>
                {APP_PW_PROTOCOLS.filter(p => r[p]).map(p => t('mailboxes.appPw.protocol.' + p)).join(', ') || t('mailboxes.appPw.noProtocols')}
              </div>
            </div>
            <div className="mf-row" style={{ gap: 2, flex: 'none' }}>
              <Button variant="ghost" size="sm" onClick={() => startEdit(r)} disabled={busy}>{t('common.edit')}</Button>
              <Button variant="ghost" size="sm" onClick={() => setConfirmDelete(r)} disabled={busy}>{t('common.delete')}</Button>
            </div>
          </div>
        ))
      )}
      <div style={{ marginTop: 12 }}>
        {editingId != null && (
          <div className="mf-u-faint" style={{ fontSize: 11.5, marginBottom: 8, lineHeight: 1.5 }}>{t('mailboxes.appPw.editingHint')}</div>
        )}
        <Input placeholder={t('mailboxes.appPw.namePlaceholder')} value={name} onChange={e => setName(e.target.value)} style={{ width: '100%' }} />
        <div className="mf-row" style={{ gap: 8, marginTop: 8 }}>
          <Input className="mf-spacer" type="text" mono placeholder={t('mailboxes.appPw.pwPlaceholder')} value={pw} onChange={e => setPw(e.target.value)} />
          <Button variant="secondary" size="sm" onClick={() => setPw(randomPassword())}>{t('mailboxes.form.generate')}</Button>
        </div>
        <ProtocolPicker selected={protocols} onToggle={toggleProtocol} />
        <div className="mf-row mf-row--between" style={{ marginTop: 10 }}>
          {editingId != null
            ? <Button variant="link" size="sm" onClick={resetForm}>{t('common.cancel')}</Button>
            : <span />}
          <Button variant="primary" size="sm" onClick={submit} disabled={busy}>
            {editingId != null ? t('common.save') : t('common.add')}
          </Button>
        </div>
      </div>
      {confirmDelete && (
        <ConfirmModal
          title={t('mailboxes.appPw.deleteTitle')}
          msg={t('mailboxes.appPw.deleteMsg', { name: confirmDelete.name || t('mailboxes.appPw.unnamed') })}
          cta={t('common.delete')}
          danger
          onCancel={() => setConfirmDelete(null)}
          onConfirm={doDelete}
        />
      )}
    </Section>
  );
}

/**
 * Sieve filters — server-side mail rules. The collection endpoint returns every
 * filter, so the mailbox's own rows are selected client-side by username.
 */
function FiltersSection({ mailbox }) {
  const t = useT();
  const { toast } = useToast();
  const { data, loading, reload } = useApi('/api/filters', []);
  const [desc, setDesc] = useState('');
  const [type, setType] = useState('postfilter');
  const [script, setScript] = useState('');
  const [busy, setBusy] = useState(false);
  const rows = asList(data).filter(f => f.username === mailbox);

  async function add() {
    if (busy) return;
    if (!script.trim()) { toast(t('mailboxes.filters.invalid')); return; }
    setBusy(true);
    try {
      await api.post('/api/filters', {
        username: mailbox,
        script_desc: desc.trim(),
        filter_type: type,
        script_data: script,
        active: '1',
      });
      toast(t('mailboxes.filters.added'));
      setDesc(''); setScript('');
      reload();
    } catch (err) {
      toast(t('mailboxes.filters.failed'), errText(err, ''));
    } finally { setBusy(false); }
  }

  async function remove(id) {
    setBusy(true);
    try {
      await api.del('/api/filters', { items: [String(id)] });
      toast(t('mailboxes.filters.removed'));
      reload();
    } catch (err) {
      toast(t('mailboxes.filters.failed'), errText(err, ''));
    } finally { setBusy(false); }
  }

  return (
    <Section title={t('mailboxes.filters.title')} hint={t('mailboxes.filters.hint')}>
      {loading ? (
        <div className="mf-u-faint" style={{ fontSize: 12.5 }}>{t('common.loading')}</div>
      ) : rows.length === 0 ? (
        <div className="mf-u-faint" style={{ fontSize: 12.5 }}>{t('mailboxes.filters.empty')}</div>
      ) : (
        rows.map(r => (
          <SectionRow key={r.id} onRemove={() => remove(r.id)} removeLabel={t('common.delete')} busy={busy}>
            <div className="mf-truncate" style={{ fontSize: 13, color: 'var(--ink)' }}>{r.script_desc || t('mailboxes.filters.unnamed')}</div>
            <div className="mf-u-faint" style={{ fontSize: 11 }}>{r.filter_type}</div>
          </SectionRow>
        ))
      )}
      <div style={{ marginTop: 12 }}>
        <Input placeholder={t('mailboxes.filters.descPlaceholder')} value={desc} onChange={e => setDesc(e.target.value)} style={{ width: '100%' }} />
        <select className="mf-input" style={{ width: '100%', marginTop: 8 }} value={type} onChange={e => setType(e.target.value)}>
          <option value="prefilter">{t('mailboxes.filters.typePre')}</option>
          <option value="postfilter">{t('mailboxes.filters.typePost')}</option>
        </select>
        <textarea
          className="mf-input"
          style={{ marginTop: 8, minHeight: 90, fontFamily: 'var(--font-mono)', fontSize: 12.5, width: '100%' }}
          placeholder={t('mailboxes.filters.scriptPlaceholder')}
          value={script}
          onChange={e => setScript(e.target.value)}
        />
        <div className="mf-row mf-row--between" style={{ marginTop: 8 }}>
          <span />
          <Button variant="primary" size="sm" onClick={add} disabled={busy}>{t('common.add')}</Button>
        </div>
      </div>
    </Section>
  );
}

/**
 * Per-mailbox rate limit — caps outbound messages within a rolling window.
 * mailcow edits this in place, so there is no add/remove, only a value + frame.
 */
function RateLimitSection({ mailbox }) {
  const t = useT();
  const { toast } = useToast();
  const { data, loading, reload } = useApi('/api/ratelimits/mailbox', []);
  const [value, setValue] = useState('');
  const [frame, setFrame] = useState('s');
  const [busy, setBusy] = useState(false);
  const [seeded, setSeeded] = useState(false);

  // Seed the inputs from this mailbox's current limit once the list arrives.
  const mine = normalizeRows(data).find(r => r.username === mailbox || r.mailbox === mailbox);
  if (!seeded && !loading && data != null) {
    if (mine) {
      setValue(mine.rl_value != null ? String(mine.rl_value) : '');
      if (mine.rl_frame) setFrame(mine.rl_frame);
    }
    setSeeded(true);
  }

  async function save() {
    if (busy) return;
    setBusy(true);
    try {
      await api.put('/api/ratelimits/mailbox', {
        items: [mailbox],
        attr: { rl_value: value === '' ? '' : Number(value), rl_frame: frame },
      });
      toast(t('mailboxes.rl.saved'));
      reload();
    } catch (err) {
      toast(t('mailboxes.rl.failed'), errText(err, ''));
    } finally { setBusy(false); }
  }

  return (
    <Section title={t('mailboxes.rl.title')} hint={t('mailboxes.rl.hint')}>
      {loading ? (
        <div className="mf-u-faint" style={{ fontSize: 12.5 }}>{t('common.loading')}</div>
      ) : (
        <div className="mf-row" style={{ gap: 8, alignItems: 'flex-end' }}>
          <FormField label={t('mailboxes.rl.value')} style={{ flex: 1 }}>
            <Input type="number" min="0" align="right" placeholder="0" value={value} onChange={e => setValue(e.target.value)} />
          </FormField>
          <FormField label={t('mailboxes.rl.frame')} style={{ flex: 1 }}>
            <select className="mf-input" value={frame} onChange={e => setFrame(e.target.value)}>
              <option value="s">{t('mailboxes.rl.perSecond')}</option>
              <option value="m">{t('mailboxes.rl.perMinute')}</option>
              <option value="h">{t('mailboxes.rl.perHour')}</option>
              <option value="d">{t('mailboxes.rl.perDay')}</option>
            </select>
          </FormField>
          <Button variant="primary" size="sm" onClick={save} disabled={busy}>{t('common.save')}</Button>
        </div>
      )}
    </Section>
  );
}

/**
 * Temporary (throwaway) aliases — short-lived addresses that forward to the
 * mailbox. mailcow generates the address; we only list, add and remove them.
 */
function TempAliasesSection({ mailbox }) {
  const t = useT();
  const { toast } = useToast();
  const { data, loading, reload } = useApi('/api/temp-aliases/' + encodeURIComponent(mailbox), [mailbox]);
  const [busy, setBusy] = useState(false);
  const rows = asList(data);

  async function add() {
    if (busy) return;
    setBusy(true);
    try {
      await api.post('/api/temp-aliases', { username: mailbox });
      toast(t('mailboxes.tempAlias.added'));
      reload();
    } catch (err) {
      toast(t('mailboxes.tempAlias.failed'), errText(err, ''));
    } finally { setBusy(false); }
  }

  async function remove(address) {
    setBusy(true);
    try {
      await api.del('/api/temp-aliases', { items: [address] });
      toast(t('mailboxes.tempAlias.removed'));
      reload();
    } catch (err) {
      toast(t('mailboxes.tempAlias.failed'), errText(err, ''));
    } finally { setBusy(false); }
  }

  return (
    <Section title={t('mailboxes.tempAlias.title')} hint={t('mailboxes.tempAlias.hint')}>
      {loading ? (
        <div className="mf-u-faint" style={{ fontSize: 12.5 }}>{t('common.loading')}</div>
      ) : rows.length === 0 ? (
        <div className="mf-u-faint" style={{ fontSize: 12.5 }}>{t('mailboxes.tempAlias.empty')}</div>
      ) : (
        rows.map(r => (
          <SectionRow key={r.address} onRemove={() => remove(r.address)} removeLabel={t('common.delete')} busy={busy}>
            <span className="mf-u-mono mf-truncate" style={{ fontSize: 12.5, color: 'var(--ink)' }}>{r.address}</span>
          </SectionRow>
        ))
      )}
      <div className="mf-row mf-row--between" style={{ marginTop: 12 }}>
        <span />
        <Button variant="primary" size="sm" onClick={add} disabled={busy}>{t('mailboxes.tempAlias.add')}</Button>
      </div>
    </Section>
  );
}

// Standard mailcow client ports: IMAP over implicit TLS, and SMTP submission
// with STARTTLS — the combination mailcow's own docs recommend for a new
// mail client, and stable across deployments (mailcow rarely reassigns them).
const IMAP_PORT = 993;
const SMTP_PORT = 587;

/**
 * Client settings — the host/port/username an admin needs to hand off to
 * whoever is configuring a mail client (Outlook, Thunderbird, a phone) for
 * this mailbox. The password is deliberately never shown here: Mailfold
 * never retains a mailbox's plaintext password past the request that set
 * it, so there is nothing to reveal beyond what was just typed into the
 * reset field above. Hidden entirely when MAILFOLD_SERVER_NAME isn't
 * configured, matching the topbar status indicator's own precedent.
 */
function ClientSettingsSection({ mailbox }) {
  const t = useT();
  const { toast } = useToast();
  const { data } = useApi('/api/status/server', []);
  const server = data && data.name;
  if (!server) return null;

  const summary = [
    `${t('mailboxes.clientSettings.username')}: ${mailbox}`,
    `${t('mailboxes.clientSettings.imap')}: ${server}:${IMAP_PORT} (SSL/TLS)`,
    `${t('mailboxes.clientSettings.smtp')}: ${server}:${SMTP_PORT} (STARTTLS)`,
  ].join('\n');

  async function copyAll() {
    try {
      await navigator.clipboard.writeText(summary);
      toast(t('mailboxes.clientSettings.copied'));
    } catch {
      /* clipboard may be unavailable; the values are still shown below */
    }
  }

  return (
    <Section title={t('mailboxes.clientSettings.title')} hint={t('mailboxes.clientSettings.hint')}>
      <div style={{ fontSize: 12.5, lineHeight: 2 }}>
        <div className="mf-row mf-row--between">
          <span className="mf-u-faint">{t('mailboxes.clientSettings.username')}</span>
          <span className="mf-u-mono" style={{ color: 'var(--ink)' }}>{mailbox}</span>
        </div>
        <div className="mf-row mf-row--between">
          <span className="mf-u-faint">{t('mailboxes.clientSettings.imap')}</span>
          <span className="mf-u-mono" style={{ color: 'var(--ink)' }}>{server}:{IMAP_PORT} · SSL/TLS</span>
        </div>
        <div className="mf-row mf-row--between">
          <span className="mf-u-faint">{t('mailboxes.clientSettings.smtp')}</span>
          <span className="mf-u-mono" style={{ color: 'var(--ink)' }}>{server}:{SMTP_PORT} · STARTTLS</span>
        </div>
      </div>
      <Button variant="secondary" size="sm" onClick={copyAll} style={{ marginTop: 10 }}>{t('mailboxes.clientSettings.copy')}</Button>
    </Section>
  );
}

/**
 * Create / edit a mailbox in a right-hand slide-over.
 *   mode      'create' | 'edit'
 *   mailbox   the row (edit mode)
 *   domains   [{domain_name}]  for the create form's domain picker
 *   onClose   () => void
 *   onSaved   () => void   — parent refetches the list
 */
export function MailboxDrawer({ mode, mailbox, domains = [], onClose, onSaved, onDelete }) {
  const t = useT();
  const { toast } = useToast();
  const editing = mode === 'edit';

  const [localPart, setLocalPart] = useState('');
  const [domain, setDomain] = useState(domains[0] ? domains[0].domain_name : '');
  const [name, setName] = useState(editing ? mailbox.name || '' : '');
  const [quotaGB, setQuotaGB] = useState(editing ? bytesToGB(mailbox.quota) : 3);
  const [password, setPassword] = useState('');
  const [active, setActive] = useState(editing ? isActive(mailbox.active) : true);
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (busy) return;
    if (!editing && (!localPart.trim() || !domain || password.length < 8)) {
      toast(t('mailboxes.form.invalid'));
      return;
    }
    setBusy(true);
    try {
      if (editing) {
        const attr = { name, quota: quotaGB * 1024, active: active ? '1' : '0' };
        if (password) {
          attr.password = password;
          attr.password2 = password;
        }
        await api.put('/api/mailboxes', { items: [mailbox.username], attr });
        toast(t('mailboxes.form.updated', { mailbox: decodeIdnAddress(mailbox.username) }));
      } else {
        await api.post('/api/mailboxes', {
          local_part: localPart.trim(),
          domain,
          name,
          quota: quotaGB * 1024,
          password,
          password2: password,
          active: active ? '1' : '0',
        });
        toast(t('mailboxes.form.created', { mailbox: decodeIdnAddress(localPart.trim() + '@' + domain) }));
      }
      onSaved();
      onClose();
    } catch (err) {
      toast(t('mailboxes.form.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  const addr = decodeIdnAddress(editing ? mailbox.username : (localPart ? localPart + '@' + domain : ''));
  const footer = (
    <>
      {editing && onDelete && (
        <Button variant="danger" onClick={() => onDelete(mailbox)}>{t('common.delete')}</Button>
      )}
      <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
      <Button variant="primary" onClick={submit} disabled={busy}>
        {busy ? t('mailboxes.form.saving') : (editing ? t('common.save') : t('mailboxes.form.create'))}
      </Button>
    </>
  );

  return (
    <Drawer
      title={editing ? t('mailboxes.form.editTitle') : t('mailboxes.form.newTitle')}
      subtitle={addr}
      icon={<Avatar size={38}>{initials(name || addr || '?')}</Avatar>}
      footer={footer}
      onClose={onClose}
    >
      {!editing && (
        <div className="mf-row" style={{ gap: 10 }}>
          <FormField label={t('mailboxes.form.localPart')} style={{ flex: 1 }}>
            <Input placeholder="jane" value={localPart} onChange={e => setLocalPart(e.target.value)} />
          </FormField>
          <FormField label={t('mailboxes.form.domain')} style={{ flex: 1 }}>
            <select className="mf-input" value={domain} onChange={e => setDomain(e.target.value)}>
              {domains.map(d => <option key={d.domain_name} value={d.domain_name}>{decodeIdnDomain(d.domain_name)}</option>)}
            </select>
          </FormField>
        </div>
      )}

      <FormField label={t('mailboxes.form.name')}>
        <Input placeholder="Jane Doe" value={name} onChange={e => setName(e.target.value)} />
      </FormField>

      <FormField label={t('mailboxes.form.quota')}>
        <Input type="number" min="1" align="right" value={quotaGB} onChange={e => setQuotaGB(Number(e.target.value) || 1)} />
      </FormField>

      <FormField label={editing ? t('mailboxes.form.resetPassword') : t('mailboxes.form.password')}>
        <div className="mf-row" style={{ gap: 8 }}>
          <Input
            className="mf-spacer"
            type="text"
            mono
            placeholder={editing ? t('mailboxes.form.leaveBlank') : '••••••••'}
            value={password}
            onChange={e => setPassword(e.target.value)}
          />
          <Button variant="secondary" size="sm" onClick={() => setPassword(randomPassword())}>{t('mailboxes.form.generate')}</Button>
        </div>
      </FormField>

      <div className="mf-row mf-row--between" style={{ marginTop: 6 }}>
        <span className="mf-u-muted" style={{ fontSize: 13 }}>{t('mailboxes.form.active')}</span>
        <Toggle on={active} onClick={() => setActive(a => !a)} style={{ cursor: 'pointer' }} />
      </div>

      {editing && (
        <>
          <ClientSettingsSection mailbox={mailbox.username} />
          <AppPasswordsSection mailbox={mailbox.username} />
          <FiltersSection mailbox={mailbox.username} />
          <RateLimitSection mailbox={mailbox.username} />
          <TempAliasesSection mailbox={mailbox.username} />
        </>
      )}
    </Drawer>
  );
}
