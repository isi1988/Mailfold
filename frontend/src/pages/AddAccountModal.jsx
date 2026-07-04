import React, { useState } from 'react';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { PasswordField } from '../components/PasswordField.jsx';
import { useWebmailAuth } from '../auth/WebmailAuthContext.jsx';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';
import { wm } from '../api/webmail.js';

// Quick-pick IMAP presets for common providers.
const PROVIDERS = [
  { name: 'Gmail', host: 'imap.gmail.com', port: '993', encryption: 'SSL' },
  { name: 'Yandex', host: 'imap.yandex.com', port: '993', encryption: 'SSL' },
  { name: 'Mail.ru', host: 'imap.mail.ru', port: '993', encryption: 'SSL' },
  { name: 'Outlook', host: 'outlook.office365.com', port: '993', encryption: 'SSL' },
  { name: 'Exchange', host: '', port: '993', encryption: 'SSL', placeholder: 'mail.company.com' },
];

function TabBtn({ active, onClick, children }) {
  return (
    <button
      onClick={onClick}
      style={{
        flex: 1, padding: '8px 10px', borderRadius: 8,
        border: '1px solid ' + (active ? 'var(--accent)' : 'var(--hair)'),
        background: active ? 'var(--accent-soft)' : 'transparent',
        color: active ? 'var(--accent-ink)' : 'var(--muted)',
        font: '600 12.5px system-ui', cursor: 'pointer',
      }}
    >{children}</button>
  );
}

/**
 * Add-account slide-over: log into another Mailfold mailbox (a switchable
 * account) or connect an external mailbox to sync into the current one.
 */
export function AddAccountModal({ onClose }) {
  const t = useT();
  const { toast } = useToast();
  const { login, verifyLogin2FA } = useWebmailAuth();
  const [tab, setTab] = useState('mailfold');
  const [busy, setBusy] = useState(false);

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [pending2FA, setPending2FA] = useState(null); // { pendingToken } once the mailbox's own 2FA is required
  const [code, setCode] = useState('');
  const [codeError, setCodeError] = useState('');

  const [ext, setExt] = useState({ host: '', port: '993', user: '', password: '', encryption: 'SSL', interval: '15' });
  const [sel, setSel] = useState(''); // selected provider preset name
  const setE = (k, v) => setExt(x => ({ ...x, [k]: v }));

  async function addLocal() {
    if (busy) return;
    setBusy(true); setError('');
    try {
      const res = await login(email.trim(), password); // adds and switches to the new account
      if (res && res.needs_2fa) {
        setPending2FA({ pendingToken: res.pending_token });
        setBusy(false);
        return;
      }
      toast(t('webmail.account.added', { email: email.trim() }));
      onClose();
    } catch (e) {
      setError(e && e.status === 401 ? t('webmail.invalid') : (e.message || t('webmail.invalid')));
    } finally {
      setBusy(false);
    }
  }

  async function submitCode() {
    if (busy || !code.trim()) return;
    setBusy(true); setCodeError('');
    try {
      await verifyLogin2FA(pending2FA.pendingToken, code.trim(), email.trim());
      toast(t('webmail.account.added', { email: email.trim() }));
      onClose();
    } catch {
      setCodeError(t('login.twoFactor.invalidCode'));
    } finally {
      setBusy(false);
    }
  }

  async function connectExt() {
    if (busy) return;
    if (!ext.host.trim() || !ext.user.trim()) { toast(t('webmail.account.extInvalid')); return; }
    setBusy(true);
    try {
      await wm.connectExternal({ ...ext, interval: Number(ext.interval) || 15 });
      toast(t('webmail.account.connected'));
      onClose();
    } catch (e) {
      toast(t('webmail.account.connectFailed'), (e && e.body && e.body.error) || (e && e.message) || '');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mf-overlay mf-overlay--right" onClick={onClose}>
      <div className="mf-drawer" style={{ display: 'flex', flexDirection: 'column' }} onClick={e => e.stopPropagation()}>
        <div className="mf-drawer__head">
          <div className="mf-drawer__title">{t('webmail.account.add')}</div>
          <div className="mf-modal-close mf-spacer" onClick={onClose}><Icon name="close" size={18} /></div>
        </div>

        <div style={{ padding: '8px 20px 0' }}>
          <div className="mf-u-muted" style={{ fontSize: 12.5, margin: '2px 0 14px', lineHeight: 1.5 }}>{t('webmail.account.addSub')}</div>
          <div style={{ display: 'flex', gap: 6, marginBottom: 16 }}>
            <TabBtn active={tab === 'mailfold'} onClick={() => setTab('mailfold')}>{t('webmail.account.tabMailfold')}</TabBtn>
            <TabBtn active={tab === 'external'} onClick={() => setTab('external')}>{t('webmail.account.tabExternal')}</TabBtn>
          </div>
        </div>

        <div style={{ padding: '0 20px', overflow: 'auto', flex: 1 }}>
          {tab === 'mailfold' ? (
            pending2FA ? (
              <>
                <FormField label={t('login.twoFactor.codeLabel')}>
                  <Input mono autoFocus placeholder={t('login.twoFactor.codePlaceholder')} autoComplete="one-time-code" value={code} onChange={e => setCode(e.target.value)} />
                </FormField>
                {codeError && <div className="mf-form-error" style={{ marginTop: 10 }} role="alert">{codeError}</div>}
              </>
            ) : (
            <>
              <FormField label={t('webmail.mailbox')}>
                <Input placeholder="you@example.com" autoComplete="username" value={email} onChange={e => setEmail(e.target.value)} />
              </FormField>
              <FormField label={t('webmail.password')}>
                <PasswordField value={password} onChange={e => setPassword(e.target.value)} />
              </FormField>
              {error && <div className="mf-form-error" style={{ marginTop: 10 }} role="alert">{error}</div>}
            </>
            )
          ) : (
            <>
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 14 }}>
                {PROVIDERS.map(p => {
                  const on = sel === p.name;
                  return (
                    <button key={p.name} onClick={() => { setSel(p.name); setExt(x => ({ ...x, host: p.host, port: p.port, encryption: p.encryption })); }}
                      style={{ padding: '7px 12px', borderRadius: 8, border: '1px solid ' + (on ? 'var(--accent)' : 'var(--hair)'), background: on ? 'var(--accent-soft)' : 'transparent', color: on ? 'var(--accent-ink)' : 'var(--muted)', font: '600 12.5px system-ui', cursor: 'pointer' }}>
                      {p.name}
                    </button>
                  );
                })}
              </div>
              <FormField label={t('webmail.account.imapHost')}>
                <Input placeholder={(PROVIDERS.find(p => p.name === sel) || {}).placeholder || 'imap.gmail.com'} value={ext.host} onChange={e => { setE('host', e.target.value); setSel(''); }} />
              </FormField>
              <div className="mf-row" style={{ gap: 10 }}>
                <FormField label={t('webmail.account.port')} style={{ width: 96 }}>
                  <Input value={ext.port} onChange={e => setE('port', e.target.value)} />
                </FormField>
                <FormField label={t('webmail.account.encryption')} style={{ flex: 1 }}>
                  <select className="mf-input" value={ext.encryption} onChange={e => setE('encryption', e.target.value)}>
                    <option value="SSL">SSL</option>
                    <option value="TLS">STARTTLS</option>
                    <option value="PLAIN">{t('webmail.account.encNone')}</option>
                  </select>
                </FormField>
              </div>
              <FormField label={t('webmail.account.username')}>
                <Input placeholder="you@gmail.com" value={ext.user} onChange={e => setE('user', e.target.value)} />
              </FormField>
              <FormField label={t('webmail.password')}>
                <PasswordField value={ext.password} onChange={e => setE('password', e.target.value)} />
              </FormField>
              <FormField label={t('webmail.account.syncEvery')}>
                <select className="mf-input" value={ext.interval} onChange={e => setE('interval', e.target.value)}>
                  <option value="5">{t('webmail.account.min5')}</option>
                  <option value="15">{t('webmail.account.min15')}</option>
                  <option value="30">{t('webmail.account.min30')}</option>
                  <option value="60">{t('webmail.account.hour1')}</option>
                </select>
              </FormField>
              <div className="mf-u-faint" style={{ fontSize: 12, marginTop: 8, lineHeight: 1.5 }}>{t('webmail.account.extHint')}</div>
            </>
          )}
        </div>

        <div className="mf-drawer__foot">
          {tab === 'mailfold'
            ? (pending2FA
              ? <Button variant="primary" onClick={submitCode} disabled={busy}>{busy ? t('login.twoFactor.verifying') : t('login.twoFactor.verify')}</Button>
              : <Button variant="primary" onClick={addLocal} disabled={busy}>{busy ? t('webmail.signingIn') : t('webmail.account.addBtn')}</Button>)
            : <Button variant="primary" onClick={connectExt} disabled={busy}>{busy ? t('webmail.account.connecting') : t('webmail.account.connectBtn')}</Button>}
          <Button variant="link" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
        </div>
      </div>
    </div>
  );
}
