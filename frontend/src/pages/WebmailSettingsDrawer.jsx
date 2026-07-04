import React, { useEffect, useState } from 'react';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { Tabs } from '../ds/components/molecules/Tabs.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Textarea } from '../ds/components/atoms/Textarea.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Toggle } from '../ds/components/atoms/Toggle.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { PasswordField } from '../components/PasswordField.jsx';
import { RecoveryCodesBox } from './RecoveryCodesBox.jsx';
import { ConfirmModal } from '../ds/components/organisms/ConfirmModal.jsx';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';
import { wm } from '../api/webmail.js';

function errText(err, fallback) {
  return (err && err.body && err.body.error) || (err && err.message) || fallback;
}

// --- Signature tab ---

function SignatureTab() {
  const t = useT();
  const { toast } = useToast();
  const [value, setValue] = useState('');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    wm.signature.get()
      .then(res => { if (!cancelled) setValue(res.signature || ''); })
      .catch(() => {})
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, []);

  async function save() {
    if (busy) return;
    setBusy(true);
    try {
      await wm.signature.set(value);
      toast(t('webmail.settings.signature.saved'));
    } catch (err) {
      toast(t('webmail.settings.signature.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  if (loading) return <div className="mf-u-muted" style={{ fontSize: 13, padding: '10px 0' }}>{t('common.loading')}</div>;

  return (
    <>
      <div className="mf-u-faint" style={{ fontSize: 12.5, marginBottom: 12, lineHeight: 1.5 }}>{t('webmail.settings.signature.hint')}</div>
      <Textarea rows={8} value={value} onChange={e => setValue(e.target.value)} style={{ width: '100%' }} />
      <Button variant="primary" onClick={save} disabled={busy} style={{ marginTop: 12 }}>{busy ? t('common.saving') : t('common.save')}</Button>
    </>
  );
}

// --- Rules tab ---

const RULE_FIELDS = ['from', 'to', 'subject'];

function RuleForm({ initial, folders, onSave, onCancel, busy }) {
  const t = useT();
  const [description, setDescription] = useState((initial && initial.description) || '');
  const [field, setField] = useState((initial && initial.field) || 'from');
  const [value, setValue] = useState((initial && initial.value) || '');
  const [targetFolder, setTargetFolder] = useState((initial && initial.target_folder) || folders[0] || '');
  const [active, setActive] = useState(initial ? initial.active : true);

  const folderOptions = folders.includes(targetFolder) || !targetFolder ? folders : [targetFolder, ...folders];

  return (
    <div style={{ border: '1px solid var(--hair)', borderRadius: 10, padding: 14, marginBottom: 12 }}>
      <FormField label={t('webmail.settings.rules.description')}>
        <Input value={description} onChange={e => setDescription(e.target.value)} placeholder={t('webmail.settings.rules.descriptionPlaceholder')} />
      </FormField>
      <div className="mf-row" style={{ gap: 10 }}>
        <FormField label={t('webmail.settings.rules.field')} style={{ flex: 1 }}>
          <select className="mf-input" value={field} onChange={e => setField(e.target.value)}>
            {RULE_FIELDS.map(f => <option key={f} value={f}>{t('webmail.settings.rules.fields.' + f)}</option>)}
          </select>
        </FormField>
        <FormField label={t('webmail.settings.rules.contains')} style={{ flex: 1 }}>
          <Input value={value} onChange={e => setValue(e.target.value)} />
        </FormField>
      </div>
      <FormField label={t('webmail.settings.rules.moveTo')}>
        <select className="mf-input" value={targetFolder} onChange={e => setTargetFolder(e.target.value)}>
          {folderOptions.map(f => <option key={f} value={f}>{f}</option>)}
        </select>
      </FormField>
      <div className="mf-row mf-row--between" style={{ marginTop: 4 }}>
        <span className="mf-u-muted" style={{ fontSize: 13 }}>{t('common.active')}</span>
        <Toggle on={active} onClick={() => setActive(a => !a)} style={{ cursor: 'pointer' }} />
      </div>
      <div className="mf-row" style={{ gap: 8, marginTop: 12, justifyContent: 'flex-end' }}>
        <Button variant="secondary" size="sm" onClick={onCancel}>{t('common.cancel')}</Button>
        <Button
          variant="primary" size="sm" disabled={busy || !value.trim() || !targetFolder.trim()}
          onClick={() => onSave({ description, field, value, target_folder: targetFolder, active })}
        >
          {busy ? t('common.saving') : t('common.save')}
        </Button>
      </div>
    </div>
  );
}

function RulesTab() {
  const t = useT();
  const { toast } = useToast();
  const [rules, setRules] = useState([]);
  const [folders, setFolders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState(null); // 'new' | rule object | null
  const [busy, setBusy] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(null);

  async function reload() {
    const [r, f] = await Promise.all([wm.rules.list(), wm.folders()]);
    setRules(r || []);
    setFolders((f || []).map(x => x.name));
  }

  useEffect(() => { reload().catch(() => {}).finally(() => setLoading(false)); }, []);

  async function save(values) {
    setBusy(true);
    try {
      if (editing && editing !== 'new') await wm.rules.update(editing.id, values);
      else await wm.rules.create(values);
      toast(t('webmail.settings.rules.saved'));
      setEditing(null);
      await reload();
    } catch (err) {
      toast(t('webmail.settings.rules.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  async function doDelete() {
    const rule = confirmDelete;
    setConfirmDelete(null);
    try {
      await wm.rules.del(rule.id);
      toast(t('webmail.settings.rules.deleted'));
      await reload();
    } catch (err) {
      toast(t('webmail.settings.rules.failed'), errText(err, ''));
    }
  }

  if (loading) return <div className="mf-u-muted" style={{ fontSize: 13, padding: '10px 0' }}>{t('common.loading')}</div>;

  return (
    <>
      <div className="mf-u-faint" style={{ fontSize: 12.5, marginBottom: 12, lineHeight: 1.5 }}>{t('webmail.settings.rules.hint')}</div>
      {rules.length === 0 && editing !== 'new' && (
        <div className="mf-u-muted" style={{ fontSize: 13, padding: '8px 0 14px' }}>{t('webmail.settings.rules.empty')}</div>
      )}
      {rules.map(r => (
        editing === r ? (
          <RuleForm key={r.id} initial={r} folders={folders} busy={busy} onSave={save} onCancel={() => setEditing(null)} />
        ) : (
          <div
            key={r.id} className="mf-row" style={{ gap: 10, padding: '10px 4px', borderTop: '1px solid var(--hair-soft)', cursor: 'pointer' }}
            onClick={() => setEditing(r)}
          >
            <div className="mf-min0" style={{ flex: 1 }}>
              <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>{r.description || t('webmail.settings.rules.untitled')}</div>
              <div className="mf-u-faint" style={{ fontSize: 12 }}>
                {t('webmail.settings.rules.summary', { field: t('webmail.settings.rules.fields.' + r.field), value: r.value, folder: r.target_folder })}
              </div>
            </div>
            {!r.active && <span className="mf-u-faint" style={{ fontSize: 11 }}>{t('common.disabled')}</span>}
            <Button variant="ghost" size="sm" title={t('common.delete')} onClick={e => { e.stopPropagation(); setConfirmDelete(r); }}>
              <Icon name="trash" size={15} />
            </Button>
          </div>
        )
      ))}
      {editing === 'new' ? (
        <RuleForm folders={folders} busy={busy} onSave={save} onCancel={() => setEditing(null)} />
      ) : (
        <Button variant="secondary" size="sm" onClick={() => setEditing('new')} style={{ marginTop: 12 }}>{t('webmail.settings.rules.add')}</Button>
      )}
      {confirmDelete && (
        <ConfirmModal
          title={t('webmail.settings.rules.deleteTitle')}
          msg={t('webmail.settings.rules.deleteMsg', { name: confirmDelete.description || t('webmail.settings.rules.untitled') })}
          cta={t('common.delete')}
          danger
          onCancel={() => setConfirmDelete(null)}
          onConfirm={doDelete}
        />
      )}
    </>
  );
}

// --- Security (2FA) tab ---

// WebmailTwoFactorEnrollWizard mirrors TwoFactorEnrollModal's three steps
// (confirm password, scan QR + confirm code, save recovery codes) but talks
// to /api/webmail/2fa/* and verifies the mailbox's real IMAP password rather
// than a Mailfold-managed one.
function WebmailTwoFactorEnrollWizard({ onDone, onCancel }) {
  const t = useT();
  const e = k => t('webmail.settings.security.enroll.' + k);
  const [step, setStep] = useState('confirm');
  const [password, setPassword] = useState('');
  const [enrollment, setEnrollment] = useState(null);
  const [code, setCode] = useState('');
  const [recoveryCodes, setRecoveryCodes] = useState([]);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function confirmPassword() {
    if (busy || !password) return;
    setBusy(true);
    setError('');
    try {
      const res = await wm.totp.enroll(password);
      setEnrollment(res);
      setStep('scan');
    } catch (err) {
      setError(errText(err, e('failed')));
    } finally {
      setBusy(false);
    }
  }

  async function confirmCode() {
    if (busy || !code.trim()) return;
    setBusy(true);
    setError('');
    try {
      const res = await wm.totp.confirm(code.trim());
      setRecoveryCodes(res.recovery_codes || []);
      setStep('codes');
    } catch {
      setError(e('invalidCode'));
    } finally {
      setBusy(false);
    }
  }

  if (step === 'confirm') {
    return (
      <div style={{ border: '1px solid var(--hair)', borderRadius: 10, padding: 14 }}>
        <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 10 }}>{e('titleConfirm')}</div>
        <FormField label={e('currentPassword')}>
          <PasswordField autoComplete="current-password" value={password} onChange={ev => setPassword(ev.target.value)} />
        </FormField>
        {error && <div className="mf-form-error" role="alert">{error}</div>}
        <div className="mf-row" style={{ gap: 8, marginTop: 12, justifyContent: 'flex-end' }}>
          <Button variant="secondary" size="sm" onClick={onCancel}>{t('common.cancel')}</Button>
          <Button variant="primary" size="sm" onClick={confirmPassword} disabled={busy}>{busy ? e('continuing') : e('continue')}</Button>
        </div>
      </div>
    );
  }

  if (step === 'scan') {
    return (
      <div style={{ border: '1px solid var(--hair)', borderRadius: 10, padding: 14 }}>
        <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 10 }}>{e('titleScan')}</div>
        <div className="mf-u-faint" style={{ fontSize: 12.5, marginBottom: 14, lineHeight: 1.5 }}>{e('scanHint')}</div>
        {enrollment && enrollment.qr_data_uri && (
          <div style={{ display: 'flex', justifyContent: 'center', marginBottom: 14 }}>
            <img src={enrollment.qr_data_uri} alt="" width={180} height={180} style={{ borderRadius: 10, border: '1px solid var(--hair)' }} />
          </div>
        )}
        <div style={{ fontSize: 12, marginBottom: 14 }}>
          <div className="mf-u-faint" style={{ marginBottom: 5 }}>{e('manualEntry')}</div>
          <div className="mf-u-mono" style={{ wordBreak: 'break-all', fontSize: 12.5 }}>{enrollment && enrollment.secret}</div>
        </div>
        <FormField label={e('code')}>
          <Input mono placeholder="123456" autoComplete="one-time-code" value={code} onChange={ev => setCode(ev.target.value)} />
        </FormField>
        {error && <div className="mf-form-error" role="alert">{error}</div>}
        <div className="mf-row" style={{ gap: 8, marginTop: 12, justifyContent: 'flex-end' }}>
          <Button variant="secondary" size="sm" onClick={onCancel}>{t('common.cancel')}</Button>
          <Button variant="primary" size="sm" onClick={confirmCode} disabled={busy}>{busy ? e('confirming') : e('confirm')}</Button>
        </div>
      </div>
    );
  }

  return (
    <div style={{ border: '1px solid var(--hair)', borderRadius: 10, padding: 14 }}>
      <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 10 }}>{e('titleCodes')}</div>
      <div className="mf-u-faint" style={{ fontSize: 12.5, marginBottom: 14, lineHeight: 1.5 }}>{e('codesIntro')}</div>
      <RecoveryCodesBox codes={recoveryCodes} copyLabel={e('copy')} />
      <div className="mf-row" style={{ marginTop: 12, justifyContent: 'flex-end' }}>
        <Button variant="primary" size="sm" onClick={onDone}>{e('done')}</Button>
      </div>
    </div>
  );
}

function SecurityTab() {
  const t = useT();
  const s = k => t('webmail.settings.security.' + k);
  const { toast } = useToast();
  const [enabled, setEnabled] = useState(false);
  const [loading, setLoading] = useState(true);
  const [enrolling, setEnrolling] = useState(false);
  const [disabling, setDisabling] = useState(false);
  const [disablePassword, setDisablePassword] = useState('');
  const [disableError, setDisableError] = useState('');
  const [busy, setBusy] = useState(false);

  async function reload() {
    const res = await wm.totp.status();
    setEnabled(!!res.enabled);
  }
  useEffect(() => { reload().catch(() => {}).finally(() => setLoading(false)); }, []);

  async function finishEnroll() {
    setEnrolling(false);
    await reload();
  }

  async function submitDisable() {
    if (busy || !disablePassword) return;
    setBusy(true);
    setDisableError('');
    try {
      await wm.totp.disable(disablePassword);
      setDisabling(false);
      setDisablePassword('');
      toast(s('disabled'));
      await reload();
    } catch (err) {
      setDisableError(errText(err, s('disableFailed')));
    } finally {
      setBusy(false);
    }
  }

  async function regenerate() {
    if (busy) return;
    setBusy(true);
    try {
      const res = await wm.totp.regenerateRecoveryCodes();
      toast(s('regenerated'));
      // Surface the fresh codes inline rather than in a toast, so they can be copied.
      setEnrolling({ regeneratedCodes: res.recovery_codes || [] });
    } catch (err) {
      toast(s('regenerateFailed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  if (loading) return <div className="mf-u-muted" style={{ fontSize: 13, padding: '10px 0' }}>{t('common.loading')}</div>;

  if (enrolling && enrolling.regeneratedCodes) {
    return (
      <div style={{ border: '1px solid var(--hair)', borderRadius: 10, padding: 14 }}>
        <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 10 }}>{s('newRecoveryCodes')}</div>
        <RecoveryCodesBox codes={enrolling.regeneratedCodes} copyLabel={t('webmail.settings.security.enroll.copy')} />
        <div className="mf-row" style={{ marginTop: 12, justifyContent: 'flex-end' }}>
          <Button variant="primary" size="sm" onClick={() => setEnrolling(false)}>{t('webmail.settings.security.enroll.done')}</Button>
        </div>
      </div>
    );
  }

  if (enrolling) {
    return <WebmailTwoFactorEnrollWizard onDone={finishEnroll} onCancel={() => setEnrolling(false)} />;
  }

  return (
    <>
      <div className="mf-u-faint" style={{ fontSize: 12.5, marginBottom: 14, lineHeight: 1.5 }}>{s('hint')}</div>
      <div className="mf-row mf-row--between" style={{ padding: '4px 0 14px' }}>
        <div>
          <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>{s('twoFactor')}</div>
          <div className="mf-u-faint" style={{ fontSize: 12 }}>{enabled ? s('enabled') : s('notEnabled')}</div>
        </div>
        {!enabled && <Button variant="primary" size="sm" onClick={() => setEnrolling(true)}>{s('enable')}</Button>}
      </div>
      {enabled && !disabling && (
        <div className="mf-row" style={{ gap: 8 }}>
          <Button variant="secondary" size="sm" onClick={regenerate} disabled={busy}>{s('regenerateCodes')}</Button>
          <Button variant="danger" size="sm" onClick={() => setDisabling(true)}>{s('disable')}</Button>
        </div>
      )}
      {disabling && (
        <div style={{ border: '1px solid var(--hair)', borderRadius: 10, padding: 14, marginTop: 8 }}>
          <FormField label={s('currentPasswordToDisable')}>
            <PasswordField autoComplete="current-password" value={disablePassword} onChange={e => setDisablePassword(e.target.value)} />
          </FormField>
          {disableError && <div className="mf-form-error" role="alert">{disableError}</div>}
          <div className="mf-row" style={{ gap: 8, marginTop: 12, justifyContent: 'flex-end' }}>
            <Button variant="secondary" size="sm" onClick={() => { setDisabling(false); setDisablePassword(''); setDisableError(''); }}>{t('common.cancel')}</Button>
            <Button variant="danger" size="sm" onClick={submitDisable} disabled={busy}>{busy ? t('common.saving') : s('disable')}</Button>
          </div>
        </div>
      )}
    </>
  );
}

// --- Drawer shell ---

export function WebmailSettingsDrawer({ onClose }) {
  const t = useT();
  return (
    <Drawer title={t('webmail.settings.title')} subtitle={t('webmail.settings.sub')} wide onClose={onClose}>
      <Tabs
        items={[
          { id: 'signature', label: t('webmail.settings.tabs.signature'), content: <SignatureTab /> },
          { id: 'rules', label: t('webmail.settings.tabs.rules'), content: <RulesTab /> },
          { id: 'security', label: t('webmail.settings.tabs.security'), content: <SecurityTab /> },
        ]}
      />
    </Drawer>
  );
}
