import React from 'react';
import { ACCOUNT, NAV, QUEUE, tone, initials } from '../data/sample.js';
import { AppShell } from '../components/organisms/AppShell.jsx';
import { PageHeader } from '../components/molecules/PageHeader.jsx';
import { Table, TableRow } from '../components/organisms/Table.jsx';
import { Pill } from '../components/atoms/Pill.jsx';
import { Button } from '../components/atoms/Button.jsx';

const account = { ...ACCOUNT, initials: initials(ACCOUNT.name) };
const COLS = [
  { label: 'Sender', w: '1.5fr' }, { label: 'Recipient', w: '1.7fr' },
  { label: 'Size', w: '.7fr' }, { label: 'Age', w: '.5fr' }, { label: 'Status', w: '.9fr' },
];

export function Queue(props) {
  return (
    <AppShell nav={NAV} current="queue" account={account} {...props}>
      <PageHeader
        title="Mail queue"
        sub="14 messages waiting · 2 deferred · oldest 48m"
        actions={<><Button variant="secondary">Flush all</Button><Button variant="danger">Delete all</Button></>}
      />
      <Table columns={COLS}>
        {QUEUE.map((q, i) => (
          <TableRow key={i}>
            <span className="mf-truncate" style={{ fontSize: 13, fontWeight: 500, color: 'var(--ink)' }}>{q.sender}</span>
            <span className="mf-u-muted mf-truncate" style={{ fontSize: 13 }}>{q.rcpt}</span>
            <span className="mf-u-mono mf-u-muted" style={{ fontSize: 12.5 }}>{q.size}</span>
            <span className="mf-u-mono mf-u-faint" style={{ fontSize: 12.5 }}>{q.age}</span>
            <span><Pill tone={tone(q.status)}>{q.status}</Pill></span>
          </TableRow>
        ))}
      </Table>
    </AppShell>
  );
}
