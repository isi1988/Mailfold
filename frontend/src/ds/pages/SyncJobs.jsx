import React from 'react';
import { ACCOUNT, NAV, SYNCJOBS, tone, initials } from '../data/sample.js';
import { AppShell } from '../components/organisms/AppShell.jsx';
import { PageHeader } from '../components/molecules/PageHeader.jsx';
import { Table, TableRow } from '../components/organisms/Table.jsx';
import { Pill } from '../components/atoms/Pill.jsx';
import { Icon } from '../components/atoms/Icon.jsx';
import { Button } from '../components/atoms/Button.jsx';

const account = { ...ACCOUNT, initials: initials(ACCOUNT.name) };
const COLS = [
  { label: 'Job', w: '1.8fr' }, { label: 'Target mailbox', w: '1.4fr' },
  { label: 'Every', w: '.7fr' }, { label: 'Last run', w: '.9fr' },
  { label: 'Status', w: '.9fr' }, { label: '', w: '18px' },
];

export function SyncJobs(props) {
  return (
    <AppShell nav={NAV} current="syncjobs" account={account} {...props}>
      <PageHeader
        title="Sync jobs"
        sub="5 IMAP/POP imports · pulling mail into Mailfold"
        actions={<Button variant="primary">+ New sync job</Button>}
      />
      <Table columns={COLS}>
        {SYNCJOBS.map((s, i) => (
          <TableRow key={i}>
            <div className="mf-min0">
              <div className="mf-truncate" style={{ fontSize: 13.5, fontWeight: 600, color: 'var(--ink)' }}>{s.name}</div>
              <div className="mf-u-faint mf-u-mono mf-truncate" style={{ fontSize: 11.5 }}>{s.src}</div>
            </div>
            <span className="mf-u-muted mf-u-mono mf-truncate" style={{ fontSize: 12.5 }}>{s.target}</span>
            <span className="mf-u-muted" style={{ fontSize: 12.5 }}>{s.every}</span>
            <span className="mf-u-faint" style={{ fontSize: 12.5 }}>{s.last}</span>
            <span><Pill tone={tone(s.status)}>{s.status}</Pill></span>
            <Icon name="chevron-right" size={14} style={{ color: 'var(--faint)' }} />
          </TableRow>
        ))}
      </Table>
    </AppShell>
  );
}
