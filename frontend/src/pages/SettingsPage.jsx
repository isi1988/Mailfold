import React, { useState } from 'react';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { Card } from '../ds/components/molecules/Card.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { ToggleRow } from '../ds/components/molecules/ToggleRow.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Segmented } from '../ds/components/atoms/Segmented.jsx';
import { useApi } from '../lib/useApi.js';
import { AsyncView } from '../components/States.jsx';
import { useToast } from '../components/Toast.jsx';
import { useT, useI18n } from '../i18n/index.jsx';
import { useAuth } from '../auth/AuthContext.jsx';

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

export function SettingsPage() {
  const t = useT();
  const { toast } = useToast();
  const { lang, setLang, locales } = useI18n();
  const { logout } = useAuth();

  const me = useApi('/api/auth/me', []);
  const version = useApi('/api/status/version', []);
  const vmail = useApi('/api/status/vmail', []);

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
        {/* Profile — bound to /api/auth/me (session only). */}
        <Card pad>
          <div className="mf-card__title" style={{ marginBottom: 16 }}>{t('settings.profile.title')}</div>
          <AsyncView loading={me.loading} error={me.error} reload={me.reload}>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
              <FormField label={t('settings.profile.username')}>
                <Input mono readonly value={(me.data && me.data.user) || ''} />
              </FormField>
              <FormField label={t('settings.profile.expires')}>
                <Input readonly value={fmtExpiry(me.data && me.data.expires_at)} />
              </FormField>
            </div>
          </AsyncView>
        </Card>

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
