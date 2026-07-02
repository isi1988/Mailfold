import React from 'react';
import { ACCOUNT, NAV, DOMAINS, DKIM, tone, initials } from '../data/sample.js';
import { AppShell } from '../components/organisms/AppShell.jsx';
import { PageHeader } from '../components/molecules/PageHeader.jsx';
import { Table, TableRow } from '../components/organisms/Table.jsx';
import { DomainDetail } from '../components/organisms/DomainDetail.jsx';
import { Logo } from '../components/atoms/Logo.jsx';
import { Pill } from '../components/atoms/Pill.jsx';
import { Icon } from '../components/atoms/Icon.jsx';
import { ProgressBar } from '../components/atoms/ProgressBar.jsx';
import { Button } from '../components/atoms/Button.jsx';

const account = { ...ACCOUNT, initials: initials(ACCOUNT.name) };
const COLS = [
  { label: 'Domain', w: '1.9fr' }, { label: 'Mailboxes', w: '.8fr' },
  { label: 'Storage', w: '1.5fr' }, { label: 'DKIM', w: '1.1fr' },
  { label: 'DNS', w: '1fr' }, { label: '', w: '18px' },
];

/** Domains list — pass `detail` to show the DKIM/DNS view for one domain. */
export function Domains({ detail = false, ...props }) {
  return (
    <AppShell nav={NAV} current="domains" account={account} {...props}>
      {detail ? (
        <DomainDetail dkim={DKIM} />
      ) : (
        <>
          <PageHeader
            title="Domains"
            sub="6 domains · 128 mailboxes · 28 aliases"
            actions={<><Button variant="secondary">Check all DNS</Button><Button variant="primary">+ Add domain</Button></>}
          />
          <Table columns={COLS}>
            {DOMAINS.map(d => (
              <TableRow key={d.name}>
                <div className="mf-cell-user">
                  <div className="mf-avatar mf-avatar--square mf-avatar--34">
                    <Logo wordmark={false} markSize={18} color="var(--accent-ink)" />
                  </div>
                  <div className="mf-min0">
                    <div className="mf-u-mono" style={{ fontSize: 14, fontWeight: 600, color: 'var(--ink)' }}>{d.name}</div>
                    <div className="mf-u-faint" style={{ fontSize: 11.5, marginTop: 2 }}>{d.aliases} aliases</div>
                  </div>
                </div>
                <span style={{ fontSize: 13.5, color: 'var(--ink)', fontWeight: 500 }}>{d.boxes}</span>
                <div>
                  <div className="mf-u-muted" style={{ fontSize: 11, marginBottom: 5 }}>{d.quota}</div>
                  <ProgressBar pct={d.pct} />
                </div>
                <span><Pill tone={d.dkim === 'active' ? 'green' : 'amber'}>{d.dkimLabel}</Pill></span>
                <span><Pill tone={d.dns === 'ok' ? 'green' : 'amber'}>{d.dnsLabel}</Pill></span>
                <Icon name="chevron-right" size={14} style={{ color: 'var(--faint)' }} />
              </TableRow>
            ))}
          </Table>
        </>
      )}
    </AppShell>
  );
}
