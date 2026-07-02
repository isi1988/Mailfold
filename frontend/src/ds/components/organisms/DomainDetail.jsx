import React from 'react';
import { cx } from '../../lib/cx.js';
import { Logo } from '../atoms/Logo.jsx';
import { Icon } from '../atoms/Icon.jsx';
import { Pill } from '../atoms/Pill.jsx';
import { Button } from '../atoms/Button.jsx';
import { Card } from '../molecules/Card.jsx';
import { StatCard } from '../molecules/StatCard.jsx';
import { Table, TableRow } from './Table.jsx';

const DNS_COLS = [
  { label: 'Type', w: '70px' }, { label: 'Host', w: '1.3fr' },
  { label: 'Value', w: '2.4fr' }, { label: 'Status', w: '90px' },
];

/** Domain detail: DKIM key + DNS records for one domain. */
export function DomainDetail({ dkim, onBack, className = '', ...rest }) {
  return (
    <div className={className} {...rest}>
      <div className="mf-row" style={{ gap: 6, fontSize: 12.5, color: 'var(--muted)', cursor: 'pointer', marginBottom: 16, display: 'inline-flex' }} onClick={onBack}>
        <Icon name="chevron-left" size={14} /> Domains
      </div>

      <div className="mf-row" style={{ gap: 14, marginBottom: 22 }}>
        <div className="mf-avatar mf-avatar--square" style={{ width: 46, height: 46, borderRadius: 12 }}>
          <Logo wordmark={false} markSize={24} color="var(--accent-ink)" />
        </div>
        <div>
          <h1 style={{ fontFamily: 'var(--font-serif)', fontSize: 28, fontWeight: 600, color: 'var(--ink-strong)' }}>{dkim.name}</h1>
          <div className="mf-row" style={{ gap: 8, marginTop: 7 }}>
            <Pill tone="green">{dkim.dkimLabel}</Pill>
            <Pill tone="green">{dkim.dnsLabel}</Pill>
          </div>
        </div>
        <div className="mf-page-head__actions">
          <Button variant="secondary">Domain settings</Button>
          <Button variant="primary">Verify DNS</Button>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(150px,1fr))', gap: 14, marginBottom: 16 }}>
        <StatCard size="sm" label="Mailboxes" value={dkim.boxes} />
        <StatCard size="sm" label="Aliases" value={dkim.aliases} />
        <StatCard size="sm" label="Storage" value={dkim.quota} />
      </div>

      <Card pad style={{ marginBottom: 16 }}>
        <div className="mf-row" style={{ marginBottom: 14 }}>
          <div>
            <span className="mf-card__title">DKIM signing key</span>
            <span className="mf-u-mono mf-u-muted" style={{ marginLeft: 10, fontSize: 12 }}>{dkim.selector}</span>
          </div>
          <Button variant="secondary" size="sm" className="mf-spacer">Rotate key</Button>
        </div>
        <div style={{ position: 'relative', background: 'var(--surface-2)', border: '1px solid var(--hair)', borderRadius: 10, padding: '14px 16px', fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--muted)', wordBreak: 'break-all', lineHeight: 1.8 }}>
          {dkim.key}
          <button className="mf-btn mf-btn--secondary mf-btn--sm" style={{ position: 'absolute', top: 10, right: 10 }}>Copy</button>
        </div>
        <div style={{ marginTop: 10, fontSize: 12, color: 'var(--faint)' }}>{dkim.bits} · {dkim.created}</div>
      </Card>

      <Table columns={DNS_COLS}>
        {dkim.records.map((r, i) => (
          <TableRow plain key={i}>
            <span style={{ font: '600 11px var(--font-mono)', color: 'var(--ink)', background: 'var(--hair-soft)', padding: '3px 8px', borderRadius: 6, justifySelf: 'start' }}>{r.type}</span>
            <span className="mf-u-mono mf-u-muted mf-truncate" style={{ fontSize: 12.5 }}>{r.host}</span>
            <span className="mf-u-mono mf-u-muted mf-truncate" style={{ fontSize: 12.5 }}>{r.value}</span>
            <span><Pill tone={r.status === 'ok' ? 'green' : 'amber'}>{r.label}</Pill></span>
          </TableRow>
        ))}
      </Table>
    </div>
  );
}
