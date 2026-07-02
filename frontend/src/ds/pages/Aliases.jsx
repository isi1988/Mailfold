import React from 'react';
import { ACCOUNT, NAV, ALIASES, tone, initials } from '../data/sample.js';
import { AppShell } from '../components/organisms/AppShell.jsx';
import { PageHeader } from '../components/molecules/PageHeader.jsx';
import { Table, TableRow } from '../components/organisms/Table.jsx';
import { Pill } from '../components/atoms/Pill.jsx';
import { Icon } from '../components/atoms/Icon.jsx';
import { Button } from '../components/atoms/Button.jsx';

const account = { ...ACCOUNT, initials: initials(ACCOUNT.name) };
const COLS = [
  { label: 'Alias', w: '1.6fr' }, { label: 'Forwards to', w: '2fr' },
  { label: 'Status', w: '.9fr' }, { label: '', w: '18px' },
];

export function Aliases(props) {
  return (
    <AppShell nav={NAV} current="aliases" account={account} {...props}>
      <PageHeader
        title="Aliases"
        sub="28 forwarding rules across 6 domains"
        actions={<Button variant="primary">+ New alias</Button>}
      />
      <Table columns={COLS}>
        {ALIASES.map(a => (
          <TableRow key={a.addr}>
            <span className="mf-u-mono mf-truncate" style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>{a.addr}</span>
            <div className="mf-row mf-min0" style={{ gap: 8 }}>
              <Icon name="arrow-right" size={14} style={{ color: 'var(--faint)', flex: 'none' }} />
              <span className="mf-u-muted mf-truncate" style={{ fontSize: 13 }}>{a.goto}</span>
            </div>
            <span><Pill tone={a.active ? 'green' : 'neutral'}>{a.statusLabel}</Pill></span>
            <Icon name="chevron-right" size={14} style={{ color: 'var(--faint)' }} />
          </TableRow>
        ))}
      </Table>
    </AppShell>
  );
}
