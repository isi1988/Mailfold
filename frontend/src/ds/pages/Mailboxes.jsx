import React from 'react';
import { ACCOUNT, NAV, MAILBOXES, tone, initials } from '../data/sample.js';
import { AppShell } from '../components/organisms/AppShell.jsx';
import { PageHeader } from '../components/molecules/PageHeader.jsx';
import { FilterTabs } from '../components/molecules/FilterTabs.jsx';
import { SearchInput } from '../components/molecules/SearchInput.jsx';
import { Table, TableRow } from '../components/organisms/Table.jsx';
import { Avatar } from '../components/atoms/Avatar.jsx';
import { Pill } from '../components/atoms/Pill.jsx';
import { Icon } from '../components/atoms/Icon.jsx';
import { ProgressBar } from '../components/atoms/ProgressBar.jsx';
import { Button } from '../components/atoms/Button.jsx';

const account = { ...ACCOUNT, initials: initials(ACCOUNT.name) };
const COLS = [
  { label: 'Mailbox', w: '2.3fr' }, { label: 'Domain', w: '1fr' },
  { label: 'Quota', w: '1.5fr' }, { label: 'Last login', w: '1fr' },
  { label: 'Status', w: '.9fr' }, { label: '', w: '18px' },
];

export function Mailboxes(props) {
  return (
    <AppShell nav={NAV} current="mailboxes" account={account} {...props}>
      <PageHeader
        title="Mailboxes"
        sub="128 mailboxes across 6 domains"
        actions={<><Button variant="secondary">Import CSV</Button><Button variant="primary">+ New mailbox</Button></>}
      />
      <div className="mf-row" style={{ marginBottom: 14 }}>
        <FilterTabs options={['All', 'Active', 'Disabled']} value="All" />
        <SearchInput className="mf-spacer" style={{ width: 250 }} placeholder="Filter mailboxes…" />
      </div>

      <Table columns={COLS}>
        {MAILBOXES.map(m => (
          <TableRow key={m.addr}>
            <div className="mf-cell-user">
              <Avatar size={34}>{m.initials}</Avatar>
              <div className="mf-min0">
                <div className="mf-cell-name mf-truncate">{m.name}</div>
                <div className="mf-cell-sub mf-truncate">{m.addr}</div>
              </div>
            </div>
            <span className="mf-u-muted" style={{ fontSize: 13 }}>{m.domain}</span>
            <div>
              <div className="mf-row mf-row--between mf-u-muted" style={{ fontSize: 11.5, marginBottom: 5 }}><span>{m.quota}</span><span>{m.pct}%</span></div>
              <ProgressBar pct={m.pct} auto />
            </div>
            <span className="mf-u-faint mf-u-mono" style={{ fontSize: 12.5 }}>{m.last}</span>
            <span><Pill tone={m.active ? tone('active') : 'neutral'}>{m.active ? 'active' : 'disabled'}</Pill></span>
            <Icon name="chevron-right" size={14} style={{ color: 'var(--faint)' }} />
          </TableRow>
        ))}
      </Table>
    </AppShell>
  );
}
