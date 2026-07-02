import React from 'react';
import { ACCOUNT, NAV, LOGS, LOG_SERVICES, tone, initials } from '../data/sample.js';
import { AppShell } from '../components/organisms/AppShell.jsx';
import { PageHeader } from '../components/molecules/PageHeader.jsx';
import { Card } from '../components/molecules/Card.jsx';
import { Tabs } from '../components/molecules/Tabs.jsx';
import { Pill } from '../components/atoms/Pill.jsx';
import { Button } from '../components/atoms/Button.jsx';

const account = { ...ACCOUNT, initials: initials(ACCOUNT.name) };

export function Logs(props) {
  return (
    <AppShell nav={NAV} current="logs" account={account} {...props}>
      <PageHeader
        title="Logs"
        sub="Live tail · mail.acme.io · all services"
        actions={
          <>
            <div className="mf-row" style={{ gap: 7, fontSize: 12.5, color: 'var(--green)', background: 'var(--green-soft)', padding: '7px 12px', borderRadius: 9, fontWeight: 600 }}>
              <span className="mf-dot mf-dot--pulse" />Live
            </div>
            <Button variant="secondary">Download</Button>
          </>
        }
      />
      <Tabs
        variant="underline"
        defaultActive="All"
        items={LOG_SERVICES.map(s => ({ id: s, label: s }))}
        style={{ marginBottom: 14 }}
      />
      <Card flush>
        {LOGS.map((l, i) => (
          <div key={i} style={{ display: 'grid', gridTemplateColumns: '76px 76px 66px 1fr', gap: 14, alignItems: 'center', padding: '9px 16px', borderTop: i ? '1px solid var(--hair-soft)' : 'none' }}>
            <span className="mf-u-mono mf-u-faint" style={{ fontSize: 12 }}>{l.time}</span>
            <span className="mf-u-mono mf-u-muted" style={{ fontSize: 11.5, justifySelf: 'start', background: 'var(--hair-soft)', padding: '2px 7px', borderRadius: 6 }}>{l.svc}</span>
            <span><Pill tone={tone(l.level)}>{l.level}</Pill></span>
            <span className="mf-u-mono mf-truncate" style={{ fontSize: 12.5, color: 'var(--ink)' }}>{l.msg}</span>
          </div>
        ))}
      </Card>
    </AppShell>
  );
}
