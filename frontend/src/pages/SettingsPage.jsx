import React, { useState } from 'react';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { Card } from '../ds/components/molecules/Card.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { ToggleRow } from '../ds/components/molecules/ToggleRow.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Textarea } from '../ds/components/atoms/Textarea.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Segmented } from '../ds/components/atoms/Segmented.jsx';
import { Pill } from '../ds/components/atoms/Pill.jsx';
import { PasswordField } from '../components/PasswordField.jsx';
import { useApi } from '../lib/useApi.js';
import { api } from '../api/client.js';
import { AsyncView } from '../components/States.jsx';
import { useToast } from '../components/Toast.jsx';
import { useT, useI18n } from '../i18n/index.jsx';
import { useAuth } from '../auth/AuthContext.jsx';
import { decodeIdnAddress } from '../lib/idn.js';
import { PasswordChangeDrawer } from './PasswordChangeDrawer.jsx';
import { TwoFactorEnrollModal } from './TwoFactorEnrollModal.jsx';
import { TwoFactorConfirmDrawer } from './TwoFactorConfirmDrawer.jsx';

// Client-only appearance preferences. Theme + accent live as data-attributes on
// <html> and are persisted so they survive a reload; there is no backend.
const THEME_KEY = 'mailfold.theme';
const ACCENT_KEY = 'mailfold.accent';
const THEMES = ['light', 'dark'];
const ACCENTS = ['ochre', 'sage', 'ink', 'clay'];
// Swatch colours mirror the light-theme --accent tokens in tokens.css.
const ACCENT_SWATCH = { ochre: '#B07C33', sage: '#4B7B58', ink: '#3C6187', clay: '#9B5A4A' };

function readTheme() {
  return localStorage.getItem(THEME_KEY) || document.documentElement.dataset.theme || 'light';
}
function readAccent() {
  return localStorage.getItem(ACCENT_KEY) || document.documentElement.dataset.accent || 'ochre';
}

// Render a mailcow session expiry (ISO string) in the reader's locale, tolerating
// a missing or unparseable value.
function fmtExpiry(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return String(iso);
  return d.toLocaleString();
}

// mailcow's /get/fail2ban returns whitelist/blacklist as newline-separated text
// (older releases) or as an array; normalise either into a textarea-friendly
// string. Editing sends the text straight back — mailcow accepts newline or
// comma separated entries.
function listToText(v) {
  if (Array.isArray(v)) return v.join('\n');
  if (v == null) return '';
  return String(v);
}

// Coerce a form field to a number for numeric mailcow tunables, tolerating an
// empty string (left untouched) by returning undefined so it is omitted.
function numOrUndef(v) {
  if (v === '' || v == null) return undefined;
  const n = Number(v);
  return Number.isNaN(n) ? undefined : n;
}

// Fail2BanSection reads the current fail2ban configuration and lets the operator
// edit the core tunables plus the whitelist/blacklist, saving via PUT.
function Fail2BanSection({ t }) {
  const { toast } = useToast();
  const f2b = useApi('/api/fail2ban', []);
  const [form, setForm] = useState(null);
  const [saving, setSaving] = useState(false);

  // Hydrate the local form once the raw config arrives, mapping mailcow's field
  // names into editable strings.
  React.useEffect(() => {
    const d = f2b.data;
    if (!d) return;
    setForm({
      ban_time: d.ban_time != null ? String(d.ban_time) : '',
      max_attempts: d.max_attempts != null ? String(d.max_attempts) : '',
      retry_window: d.retry_window != null ? String(d.retry_window) : '',
      netban_ipv4: d.netban_ipv4 != null ? String(d.netban_ipv4) : '',
      netban_ipv6: d.netban_ipv6 != null ? String(d.netban_ipv6) : '',
      whitelist: listToText(d.whitelist),
      blacklist: listToText(d.blacklist),
    });
  }, [f2b.data]);

  const set = (k, v) => setForm(prev => ({ ...(prev || {}), [k]: v }));

  // Count of live bans, if mailcow reports them, for a small read-only summary.
  const activeBans = f2b.data && f2b.data.active_bans;
  const activeCount = Array.isArray(activeBans) ? activeBans.length : undefined;

  async function save() {
    if (!form) return;
    setSaving(true);
    try {
      const attr = {
        ban_time: numOrUndef(form.ban_time),
        max_attempts: numOrUndef(form.max_attempts),
        retry_window: numOrUndef(form.retry_window),
        netban_ipv4: numOrUndef(form.netban_ipv4),
        netban_ipv6: numOrUndef(form.netban_ipv6),
        whitelist: form.whitelist,
        blacklist: form.blacklist,
      };
      await api.put('/api/fail2ban', attr);
      toast(t('settings.fail2ban.saved'));
      f2b.reload();
    } catch (err) {
      toast(t('settings.fail2ban.saveFailed'), (err && err.message) || '');
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card pad>
      <div className="mf-card__title" style={{ marginBottom: 4 }}>{t('settings.fail2ban.title')}</div>
      <div className="mf-u-faint" style={{ fontSize: 12, marginBottom: 16 }}>{t('settings.fail2ban.desc')}</div>
      <AsyncView loading={f2b.loading} error={f2b.error} reload={f2b.reload}>
        {form && (
          <>
            {activeCount != null && (
              <div className="mf-u-faint" style={{ fontSize: 12, marginBottom: 14 }}>
                {t('settings.fail2ban.activeBans', { count: activeCount })}
              </div>
            )}
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
              <FormField label={t('settings.fail2ban.banTime')}>
                <Input type="number" align="right" value={form.ban_time} onChange={e => set('ban_time', e.target.value)} />
                <div className="mf-u-faint" style={{ fontSize: 11, marginTop: 5 }}>{t('settings.fail2ban.banTimeHint')}</div>
              </FormField>
              <FormField label={t('settings.fail2ban.maxAttempts')}>
                <Input type="number" align="right" value={form.max_attempts} onChange={e => set('max_attempts', e.target.value)} />
                <div className="mf-u-faint" style={{ fontSize: 11, marginTop: 5 }}>{t('settings.fail2ban.maxAttemptsHint')}</div>
              </FormField>
              <FormField label={t('settings.fail2ban.retryWindow')}>
                <Input type="number" align="right" value={form.retry_window} onChange={e => set('retry_window', e.target.value)} />
                <div className="mf-u-faint" style={{ fontSize: 11, marginTop: 5 }}>{t('settings.fail2ban.retryWindowHint')}</div>
              </FormField>
              <FormField label={t('settings.fail2ban.netbanIpv4')}>
                <Input type="number" align="right" value={form.netban_ipv4} onChange={e => set('netban_ipv4', e.target.value)} />
                <div className="mf-u-faint" style={{ fontSize: 11, marginTop: 5 }}>{t('settings.fail2ban.netbanIpv4Hint')}</div>
              </FormField>
              <FormField label={t('settings.fail2ban.netbanIpv6')}>
                <Input type="number" align="right" value={form.netban_ipv6} onChange={e => set('netban_ipv6', e.target.value)} />
                <div className="mf-u-faint" style={{ fontSize: 11, marginTop: 5 }}>{t('settings.fail2ban.netbanIpv6Hint')}</div>
              </FormField>
            </div>
            <div style={{ height: 14 }} />
            <FormField label={t('settings.fail2ban.whitelist')}>
              <Textarea rows={4} value={form.whitelist} onChange={e => set('whitelist', e.target.value)} />
              <div className="mf-u-faint" style={{ fontSize: 11, marginTop: 5 }}>{t('settings.fail2ban.listHint')}</div>
            </FormField>
            <div style={{ height: 14 }} />
            <FormField label={t('settings.fail2ban.blacklist')}>
              <Textarea rows={4} value={form.blacklist} onChange={e => set('blacklist', e.target.value)} />
              <div className="mf-u-faint" style={{ fontSize: 11, marginTop: 5 }}>{t('settings.fail2ban.listHint')}</div>
            </FormField>
            <div className="mf-row" style={{ justifyContent: 'flex-end', marginTop: 16 }}>
              <Button variant="primary" size="sm" onClick={save} disabled={saving}>
                {saving ? t('settings.fail2ban.saving') : t('settings.fail2ban.save')}
              </Button>
            </div>
          </>
        )}
      </AsyncView>
    </Card>
  );
}

// ProfileSection shows the read-only session identity plus the editable profile
// fields backed by /api/account/profile. When that endpoint is unavailable (no
// MAILFOLD_DB_PATH), it falls back to just the read-only session fields.
function ProfileSection({ t, me }) {
  const { toast } = useToast();
  const profile = useApi('/api/account/profile', []);
  const [form, setForm] = useState(null);
  const [saving, setSaving] = useState(false);

  React.useEffect(() => {
    const d = profile.data;
    if (!d) return;
    setForm({ display_name: d.display_name || '', email: d.email || '', timezone: d.timezone || '', avatar_url: d.avatar_url || '' });
  }, [profile.data]);

  const set = (k, v) => setForm(prev => ({ ...(prev || {}), [k]: v }));
  const unavailable = profile.error && profile.error.status === 501;

  async function save() {
    if (!form) return;
    setSaving(true);
    try {
      await api.put('/api/account/profile', form);
      toast(t('settings.profile.saved'));
      profile.reload();
    } catch (err) {
      toast(t('settings.profile.saveFailed'), (err && err.message) || '');
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card pad>
      <div className="mf-card__title" style={{ marginBottom: 16 }}>{t('settings.profile.title')}</div>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
        <FormField label={t('settings.profile.username')}>
          <Input mono readonly value={(me.data && me.data.user) || ''} />
        </FormField>
        <FormField label={t('settings.profile.expires')}>
          <Input readonly value={fmtExpiry(me.data && me.data.expires_at)} />
        </FormField>
      </div>
      {unavailable ? (
        <div className="mf-u-faint" style={{ fontSize: 12, marginTop: 14 }}>{t('settings.profile.unavailable')}</div>
      ) : (
        <AsyncView loading={profile.loading} error={profile.error && profile.error.status !== 501 ? profile.error : null} reload={profile.reload}>
          {form && (
            <>
              <div style={{ height: 14 }} />
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
                <FormField label={t('settings.profile.displayName')}>
                  <Input value={form.display_name} onChange={e => set('display_name', e.target.value)} />
                </FormField>
                <FormField label={t('settings.profile.email')}>
                  <Input mono value={form.email} onChange={e => set('email', e.target.value)} />
                </FormField>
                <FormField label={t('settings.profile.timezone')}>
                  <Input placeholder="Europe/Madrid" value={form.timezone} onChange={e => set('timezone', e.target.value)} />
                </FormField>
              </div>
              <div className="mf-row" style={{ justifyContent: 'flex-end', marginTop: 16 }}>
                <Button variant="primary" size="sm" onClick={save} disabled={saving}>
                  {saving ? t('settings.profile.saving') : t('settings.profile.save')}
                </Button>
              </div>
            </>
          )}
        </AsyncView>
      )}
    </Card>
  );
}

function fmtWhen(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return d.toLocaleString();
}

// SecuritySection covers password change, two-factor auth, and the active
// session/device list — all backed by the admin-account store, so the whole
// card degrades to a muted note when that store is unavailable.
function SecuritySection({ t }) {
  const { toast } = useToast();
  const profile = useApi('/api/account/profile', []);
  const sessions = useApi('/api/account/sessions', []);
  const [modal, setModal] = useState(null); // 'password' | 'enroll' | 'disable' | 'regenerate' | null

  const unavailable = profile.error && profile.error.status === 501;
  const totpEnabled = !!(profile.data && profile.data.totp_enabled);
  const list = Array.isArray(sessions.data) ? sessions.data : [];

  async function revoke(id) {
    try {
      await api.post(`/api/account/sessions/${id}/revoke`, {});
      toast(t('settings.security.sessions.revoked'));
      sessions.reload();
    } catch (err) {
      toast(t('settings.security.sessions.failed'), (err && err.message) || '');
    }
  }

  async function revokeAll() {
    try {
      const res = await api.post('/api/account/sessions/revoke-all', {});
      toast(t('settings.security.sessions.revokedAll', { count: (res && res.revoked) || 0 }));
      sessions.reload();
    } catch (err) {
      toast(t('settings.security.sessions.failed'), (err && err.message) || '');
    }
  }

  async function disable2FA(password) {
    await api.post('/api/account/2fa/disable', { current_password: password });
    toast(t('settings.security.twoFactor.disabled'));
    profile.reload();
  }

  // Returning the response lets TwoFactorConfirmDrawer reveal the new codes
  // once, in place, the same way the enroll wizard does.
  function regenerateCodes(password) {
    return api.post('/api/account/2fa/recovery-codes', { current_password: password });
  }

  return (
    <>
      <Card style={{ padding: '6px 20px 14px' }}>
        <div style={{ padding: '14px 0 12px' }} className="mf-card__title">{t('settings.security.title')}</div>
        {unavailable ? (
          <div className="mf-u-faint" style={{ fontSize: 12, paddingBottom: 12 }}>{t('settings.security.unavailable')}</div>
        ) : (
          <>
            <ToggleRow
              title={t('settings.security.password.title')}
              desc={t('settings.security.password.desc')}
              control={<Button variant="secondary" size="sm" onClick={() => setModal('password')}>{t('settings.security.password.change')}</Button>}
            />
            <ToggleRow
              title={t('settings.security.twoFactor.title')}
              desc={totpEnabled ? t('settings.security.twoFactor.descOn') : t('settings.security.twoFactor.descOff')}
              control={(
                <div className="mf-row" style={{ gap: 8 }}>
                  <Pill tone={totpEnabled ? 'green' : 'neutral'}>
                    {totpEnabled ? t('settings.security.twoFactor.enabled') : t('settings.security.twoFactor.disabled')}
                  </Pill>
                  {totpEnabled && (
                    <Button variant="secondary" size="sm" onClick={() => setModal('regenerate')}>
                      {t('settings.security.twoFactor.regenerate')}
                    </Button>
                  )}
                  <Button variant={totpEnabled ? 'danger' : 'secondary'} size="sm" onClick={() => setModal(totpEnabled ? 'disable' : 'enroll')}>
                    {totpEnabled ? t('settings.security.twoFactor.disable') : t('settings.security.twoFactor.enable')}
                  </Button>
                </div>
              )}
            />
            <ToggleRow
              title={t('settings.security.sessions.title')}
              desc={t('settings.security.sessions.desc', { count: list.length })}
              control={<Button variant="danger" size="sm" onClick={revokeAll}>{t('settings.security.sessions.revokeAll')}</Button>}
            />
            <AsyncView loading={sessions.loading} error={sessions.error && sessions.error.status !== 501 ? sessions.error : null} reload={sessions.reload}>
              <div style={{ display: 'flex', flexDirection: 'column' }}>
                {list.map(si => (
                  <ToggleRow
                    key={si.id}
                    flush
                    title={si.user_agent || t('settings.security.sessions.unknownDevice')}
                    desc={[si.ip, fmtWhen(si.created_at)].filter(Boolean).join(' · ')}
                    control={si.current
                      ? <Pill tone="blue">{t('settings.security.sessions.current')}</Pill>
                      : <Button variant="secondary" size="sm" onClick={() => revoke(si.id)}>{t('settings.security.sessions.revoke')}</Button>}
                  />
                ))}
              </div>
            </AsyncView>
          </>
        )}
      </Card>

      {modal === 'password' && (
        <PasswordChangeDrawer
          onClose={() => setModal(null)}
          onSaved={() => { setModal(null); toast(t('settings.security.password.saved')); }}
        />
      )}
      {modal === 'enroll' && (
        <TwoFactorEnrollModal onClose={() => setModal(null)} onSaved={() => profile.reload()} />
      )}
      {modal === 'disable' && (
        <TwoFactorConfirmDrawer mode="disable" onClose={() => setModal(null)} onConfirm={disable2FA} />
      )}
      {modal === 'regenerate' && (
        <TwoFactorConfirmDrawer mode="regenerate" onClose={() => setModal(null)} onConfirm={regenerateCodes} />
      )}
    </>
  );
}

// NotifySection configures the mailbox Mailfold sends system emails (password
// resets) from. Backed by /api/account/notify-sender, gated on the admin store
// and encryption key the same way two-factor auth is.
function NotifySection({ t }) {
  const { toast } = useToast();
  const notify = useApi('/api/account/notify-sender', []);
  const [mailbox, setMailbox] = useState('');
  const [password, setPassword] = useState('');
  const [currentPassword, setCurrentPassword] = useState('');
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);

  React.useEffect(() => {
    if (notify.data) setMailbox(notify.data.mailbox || '');
  }, [notify.data]);

  const unavailable = notify.error && notify.error.status === 501;

  async function save() {
    setSaving(true);
    try {
      await api.put('/api/account/notify-sender', { mailbox: mailbox.trim(), password, current_password: currentPassword });
      toast(t('settings.notify.saved'));
      setPassword('');
      setCurrentPassword('');
      notify.reload();
    } catch (err) {
      toast(t('settings.notify.failed'), (err && err.message) || '');
    } finally {
      setSaving(false);
    }
  }

  async function test() {
    setTesting(true);
    try {
      await api.post('/api/account/notify-sender/test', {});
      toast(t('settings.notify.tested'));
    } catch (err) {
      toast(t('settings.notify.testFailed'), (err && err.message) || '');
    } finally {
      setTesting(false);
    }
  }

  return (
    <Card pad>
      <div className="mf-card__title" style={{ marginBottom: 4 }}>{t('settings.notify.title')}</div>
      <div className="mf-u-faint" style={{ fontSize: 12, marginBottom: 16 }}>{t('settings.notify.desc')}</div>
      {unavailable ? (
        <div className="mf-u-faint" style={{ fontSize: 12 }}>{t('settings.notify.unavailable')}</div>
      ) : (
        <AsyncView loading={notify.loading} error={notify.error && notify.error.status !== 501 ? notify.error : null} reload={notify.reload}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
            <FormField label={t('settings.notify.mailbox')}>
              <Input mono placeholder="noreply@example.com" value={mailbox} onChange={e => setMailbox(e.target.value)} />
            </FormField>
            <FormField label={t('settings.notify.password')}>
              <PasswordField autoComplete="off" value={password} onChange={e => setPassword(e.target.value)} />
            </FormField>
            <FormField label={t('settings.notify.currentPassword')}>
              <PasswordField autoComplete="current-password" value={currentPassword} onChange={e => setCurrentPassword(e.target.value)} />
            </FormField>
          </div>
          <div className="mf-row mf-row--between" style={{ marginTop: 16 }}>
            <span className="mf-u-faint" style={{ fontSize: 12 }}>
              {notify.data && notify.data.configured ? decodeIdnAddress(notify.data.mailbox) : t('settings.notify.notConfigured')}
            </span>
            <div className="mf-row" style={{ gap: 8 }}>
              {notify.data && notify.data.configured && (
                <Button variant="secondary" size="sm" onClick={test} disabled={testing}>
                  {testing ? t('settings.notify.testing') : t('settings.notify.test')}
                </Button>
              )}
              <Button variant="primary" size="sm" onClick={save} disabled={saving}>
                {saving ? t('settings.notify.saving') : t('settings.notify.save')}
              </Button>
            </div>
          </div>
        </AsyncView>
      )}
    </Card>
  );
}

export function SettingsPage() {
  const t = useT();
  const { toast } = useToast();
  const { lang, setLang, locales } = useI18n();
  const { logout } = useAuth();

  const me = useApi('/api/auth/me', []);
  const version = useApi('/api/status/version', []);
  const vmail = useApi('/api/status/vmail', []);
  const server = useApi('/api/status/server', []);

  const [theme, setTheme] = useState(readTheme);
  const [accent, setAccent] = useState(readAccent);

  const applyTheme = value => {
    document.documentElement.dataset.theme = value;
    localStorage.setItem(THEME_KEY, value);
    setTheme(value);
  };
  const applyAccent = value => {
    document.documentElement.dataset.accent = value;
    localStorage.setItem(ACCENT_KEY, value);
    setAccent(value);
  };

  async function signOut() {
    try {
      await logout();
      toast(t('settings.signedOut'));
    } catch (err) {
      toast(t('settings.signOutFailed'), (err && err.message) || '');
    }
  }

  // /api/status/version and /vmail proxy mailcow's raw JSON verbatim; pick the
  // fields defensively since the shape tracks the underlying mailcow release.
  const versionStr = version.data && (version.data.version || version.data.mailcow_git_version);
  const v = vmail.data || {};
  const vmailUsed = v.used;
  const vmailTotal = v.total || v.disk;
  const vmailPct = v.used_percent;

  const themeOpts = THEMES.map(x => ({ value: x, label: t('settings.appearance.theme_' + x) }));

  return (
    <>
      <PageHeader title={t('settings.title')} sub={t('settings.sub')} />

      <div style={{ maxWidth: 720, display: 'flex', flexDirection: 'column', gap: 16 }}>
        {/* Profile — read-only session fields plus the editable admin profile. */}
        <ProfileSection t={t} me={me} />

        {/* Security — password, two-factor auth, active sessions. */}
        <SecuritySection t={t} />

        {/* Appearance — client-only, no backend. */}
        <Card pad>
          <div className="mf-card__title" style={{ marginBottom: 16 }}>{t('settings.appearance.title')}</div>
          <ToggleRow
            flush
            title={t('settings.appearance.themeTitle')}
            desc={t('settings.appearance.themeDesc')}
            control={<Segmented options={themeOpts} value={theme} onSelect={applyTheme} style={{ width: 150 }} />}
          />
          <div style={{ height: 18 }} />
          <ToggleRow
            flush
            title={t('settings.appearance.accentTitle')}
            desc={t('settings.appearance.accentDesc')}
            control={(
              <div className="mf-row" style={{ gap: 9 }}>
                {ACCENTS.map(c => (
                  <span
                    key={c}
                    role="button"
                    aria-label={t('settings.appearance.accent_' + c)}
                    title={t('settings.appearance.accent_' + c)}
                    onClick={() => applyAccent(c)}
                    style={{
                      width: 26,
                      height: 26,
                      borderRadius: '50%',
                      background: ACCENT_SWATCH[c],
                      cursor: 'pointer',
                      boxShadow: c === accent ? '0 0 0 2px var(--surface),0 0 0 4px var(--accent)' : 'none',
                    }}
                  />
                ))}
              </div>
            )}
          />
        </Card>

        {/* Language — only enabled locales are offered by useI18n(). */}
        <Card pad>
          <div className="mf-card__title" style={{ marginBottom: 4 }}>{t('settings.language.title')}</div>
          <div className="mf-u-faint" style={{ fontSize: 12, marginBottom: 16 }}>{t('settings.language.desc')}</div>
          <div className="mf-row" style={{ gap: 9, flexWrap: 'wrap' }}>
            {locales.map(l => (
              <Button
                key={l.code}
                variant={l.code === lang ? 'primary' : 'secondary'}
                size="sm"
                onClick={() => setLang(l.code)}
              >
                {l.nativeName}
              </Button>
            ))}
          </div>
        </Card>

        {/* Server — read-only info from /api/status/version and /api/status/vmail. */}
        <Card style={{ padding: '6px 20px 14px' }}>
          <div style={{ padding: '14px 0 12px' }} className="mf-card__title">{t('settings.server.title')}</div>
          {server.data && server.data.name && (
            <ToggleRow
              title={t('settings.server.hostname')}
              control={<span className="mf-u-mono mf-u-muted" style={{ fontSize: 13 }}>{server.data.name}</span>}
            />
          )}
          <AsyncView loading={version.loading} error={version.error} reload={version.reload}>
            <ToggleRow
              title={t('settings.server.version')}
              control={<span className="mf-u-mono mf-u-muted" style={{ fontSize: 13 }}>{versionStr || t('settings.server.unknown')}</span>}
            />
          </AsyncView>
          <AsyncView loading={vmail.loading} error={vmail.error} reload={vmail.reload}>
            <ToggleRow
              title={t('settings.server.storage')}
              desc={vmailPct != null ? t('settings.server.storageUsed', { pct: vmailPct }) : undefined}
              control={(
                <span className="mf-u-mono mf-u-muted" style={{ fontSize: 13 }}>
                  {vmailUsed ? String(vmailUsed) + (vmailTotal ? ' / ' + String(vmailTotal) : '') : t('settings.server.unknown')}
                </span>
              )}
            />
          </AsyncView>
        </Card>

        {/* Notifications — the sender mailbox used for password-reset emails. */}
        <NotifySection t={t} />

        {/* Fail2Ban — intrusion-prevention config, bound to /api/fail2ban. */}
        <Fail2BanSection t={t} />

        {/* Sign out — ends the local session via AuthContext. */}
        <Card pad>
          <ToggleRow
            flush
            title={t('settings.signOut.title')}
            desc={t('settings.signOut.desc')}
            control={<Button variant="danger" size="sm" onClick={signOut}>{t('settings.signOut.cta')}</Button>}
          />
        </Card>
      </div>
    </>
  );
}
