import React from 'react';
import { ACCOUNT, NAV, QUARANTINE, tone, initials } from '../data/sample.js';
import { AppShell } from '../components/organisms/AppShell.jsx';
import { PageHeader } from '../components/molecules/PageHeader.jsx';
import { Table, TableRow } from '../components/organisms/Table.jsx';
import { Pill } from '../components/atoms/Pill.jsx';
import { Checkbox } from '../components/atoms/Checkbox.jsx';
import { Button } from '../components/atoms/Button.jsx';

const account = { ...ACCOUNT, initials: initials(ACCOUNT.name) };
const COLS = [
  { label: '', w: '24px' }, { label: 'Subject', w: '2.1fr' }, { label: 'Recipient', w: '1.3fr' },
  { label: 'Score', w: '.55fr' }, { label: 'Reason', w: '.9fr' }, { label: 'Held', w: '.7fr' },
];

export function Quarantine(props) {
  return (
    <AppShell nav={NAV} current="quarantine" account={account} {...props}>
      <PageHeader
        title="Quarantine"
        sub="7 messages held · auto-purge in 14 days"
        actions={<><Button variant="secondary">Release selected</Button><Button variant="danger">Delete</Button></>}
      />
      <Table columns={COLS}>
        {QUARANTINE.map((q, i) => (
          <TableRow key={i}>
            <Checkbox />
            <div className="mf-min0">
              <div className="mf-truncate" style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>{q.subj}</div>
              <div className="mf-u-faint mf-u-mono mf-truncate" style={{ fontSize: 11.5 }}>{q.from}</div>
            </div>
            <span className="mf-u-muted mf-u-mono mf-truncate" style={{ fontSize: 12.5 }}>{q.to}</span>
            <span className="mf-u-mono" style={{ fontSize: 13, fontWeight: 600, color: 'var(--' + q.scoreTone + ')' }}>{q.score}</span>
            <span><Pill tone={tone(q.reason)}>{q.reason}</Pill></span>
            <span className="mf-u-faint" style={{ fontSize: 12 }}>{q.when}</span>
          </TableRow>
        ))}
      </Table>
    </AppShell>
  );
}
