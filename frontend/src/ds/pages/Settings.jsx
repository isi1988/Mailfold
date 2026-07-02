import React from 'react';
import { ACCOUNT, NAV, initials } from '../data/sample.js';
import { AppShell } from '../components/organisms/AppShell.jsx';
import { PageHeader } from '../components/molecules/PageHeader.jsx';
import { Card } from '../components/molecules/Card.jsx';
import { FormField } from '../components/molecules/FormField.jsx';
import { ToggleRow } from '../components/molecules/ToggleRow.jsx';
import { Select } from '../components/molecules/Select.jsx';
import { Input } from '../components/atoms/Input.jsx';
import { Avatar } from '../components/atoms/Avatar.jsx';
import { Button } from '../components/atoms/Button.jsx';
import { Pill } from '../components/atoms/Pill.jsx';
import { Segmented } from '../components/atoms/Segmented.jsx';

const account = { ...ACCOUNT, initials: initials(ACCOUNT.name) };
const SUBNAV = ['Profile', 'Security', 'Appearance', 'Language & region', 'Server', 'Notifications'];
const LANGS = ['English (US)', 'English (UK)', 'Русский', 'Deutsch', 'Español', 'Français', 'Nederlands', 'Italiano', 'Polski', 'Português (BR)', 'Türkçe', 'Українська'];
const REGIONS = ['DD.MM.YYYY · 1 234,56', 'MM/DD/YYYY · 1,234.56', 'YYYY-MM-DD · 1 234.56'];
const TZS = ['Europe/Madrid — GMT+2', 'UTC — GMT+0', 'Europe/London — GMT+1', 'Europe/Moscow — GMT+3', 'America/New_York — GMT−4', 'Asia/Tokyo — GMT+9'];
const ACCENTS = ['#B07C33', '#4B7B58', '#3C6187', '#9B5A4A'];

export function Settings({ theme = 'light', ...props }) {
  return (
    <AppShell nav={NAV} current="settings" account={account} theme={theme} {...props}>
      <div style={{ marginBottom: 22 }}>
        <h1 className="mf-page-head__title">Settings</h1>
        <div className="mf-page-head__sub">Your profile, security and this Mailfold instance</div>
      </div>

      <div className="mf-row mf-row--start" style={{ gap: 26 }}>
        {/* Sub-nav */}
        <div style={{ width: 180, flex: 'none', display: 'flex', flexDirection: 'column', gap: 2 }}>
          {SUBNAV.map((s, i) => (
            <div key={s} style={i === 0
              ? { padding: '8px 12px', borderRadius: 9, font: '600 13px var(--font-sans)', color: 'var(--ink)', background: 'var(--surface)', boxShadow: '0 1px 2px rgba(0,0,0,.05),0 0 0 1px var(--hair)', cursor: 'pointer' }
              : { padding: '8px 12px', borderRadius: 9, font: '500 13px var(--font-sans)', color: 'var(--muted)', cursor: 'pointer' }}>{s}</div>
          ))}
        </div>

        {/* Panels */}
        <div className="mf-min0" style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 16 }}>
          <Card pad>
            <div className="mf-card__title" style={{ marginBottom: 16 }}>Profile</div>
            <div className="mf-row" style={{ gap: 14, marginBottom: 18 }}>
              <Avatar size={54}>JD</Avatar>
              <Button variant="secondary" size="sm">Change photo</Button>
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
              <FormField label="Display name"><Input defaultValue="Jamie Doe" /></FormField>
              <FormField label="Email"><Input mono defaultValue="jamie@acme.io" /></FormField>
              <FormField label="Role"><Input readonly defaultValue="Administrator" /></FormField>
              <FormField label="Timezone"><Input defaultValue="Europe/Madrid" /></FormField>
            </div>
          </Card>

          <Card style={{ padding: '6px 20px 14px' }}>
            <div style={{ padding: '14px 0 12px' }} className="mf-card__title">Security</div>
            <ToggleRow title="Two-factor authentication" desc="TOTP via authenticator app · enabled" control={<Pill tone="green">Enabled</Pill>} />
            <ToggleRow title="Password" desc="Last changed 3 months ago" control={<Button variant="secondary" size="sm">Change</Button>} />
            <ToggleRow title="Active sessions" desc="2 devices · Madrid, London" control={<Button variant="danger" size="sm">Sign out all</Button>} />
          </Card>

          <Card pad>
            <div className="mf-card__title" style={{ marginBottom: 16 }}>Appearance</div>
            <ToggleRow flush title="Theme" desc="Switch between warm light and warm dark" control={<Segmented options={[{ label: 'Light', value: 'light' }, { label: 'Dark', value: 'dark' }]} value={theme} style={{ width: 150 }} />} />
            <div style={{ height: 18 }} />
            <ToggleRow flush title="Accent" desc="The colour of buttons, links and highlights" control={
              <div className="mf-row" style={{ gap: 9 }}>
                {ACCENTS.map((c, i) => <span key={c} style={{ width: 26, height: 26, borderRadius: '50%', background: c, cursor: 'pointer', boxShadow: i === 0 ? '0 0 0 2px var(--surface),0 0 0 4px var(--accent)' : 'none' }} />)}
              </div>
            } />
            <div style={{ height: 18 }} />
            <ToggleRow flush title="Density" desc="Row height across tables" control={<Segmented options={['Comfortable', 'Compact']} value="Comfortable" style={{ width: 200 }} />} />
          </Card>

          <Card pad>
            <div className="mf-card__title" style={{ marginBottom: 4 }}>Language &amp; region</div>
            <div className="mf-u-faint" style={{ fontSize: 12, marginBottom: 16 }}>Applies to the Mailfold interface, dates and the webmail composer.</div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
              <FormField label="Interface language"><Select value="English (US)" options={LANGS} /></FormField>
              <FormField label="Region format"><Select value={REGIONS[0]} options={REGIONS} /></FormField>
            </div>
            <div style={{ marginTop: 16 }}>
              <FormField label="Timezone"><Select value={TZS[0]} options={TZS} /></FormField>
            </div>
            <div style={{ height: 18 }} />
            <ToggleRow flush title="Time format" desc="How times appear across the app" control={<Segmented options={['24-hour', '12-hour']} value="24-hour" style={{ width: 170 }} />} />
            <div style={{ height: 16 }} />
            <ToggleRow flush title="First day of week" desc="Affects calendar and date pickers" control={<Segmented options={['Monday', 'Sunday']} value="Monday" style={{ width: 170 }} />} />
          </Card>

          <Card style={{ padding: '6px 20px 14px' }}>
            <div style={{ padding: '14px 0 12px' }} className="mf-card__title">Server</div>
            <ToggleRow title="Hostname" control={<span className="mf-u-mono mf-u-muted" style={{ fontSize: 13 }}>mail.acme.io</span>} />
            <ToggleRow title="mailcow version" control={<div className="mf-row" style={{ gap: 9 }}><Pill tone="amber">Update available</Pill><span className="mf-u-mono mf-u-muted" style={{ fontSize: 13 }}>2025.07</span></div>} />
            <ToggleRow title="Backups" desc="Nightly at 03:00 · last ok 6h ago" control={<Pill tone="green">Healthy</Pill>} />
          </Card>
        </div>
      </div>
    </AppShell>
  );
}
