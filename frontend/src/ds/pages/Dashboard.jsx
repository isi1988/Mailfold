import React from 'react';
import { ACCOUNT, NAV, KPIS, SERVICES, CHART, CHART_DAYS, QUEUE, tone, initials } from '../data/sample.js';
import { AppShell } from '../components/organisms/AppShell.jsx';
import { PageHeader } from '../components/molecules/PageHeader.jsx';
import { StatCard } from '../components/molecules/StatCard.jsx';
import { Card } from '../components/molecules/Card.jsx';
import { Button } from '../components/atoms/Button.jsx';
import { Pill } from '../components/atoms/Pill.jsx';
import { Logo } from '../components/atoms/Logo.jsx';

const account = { ...ACCOUNT, initials: initials(ACCOUNT.name) };

export function Dashboard(props) {
  return (
    <AppShell nav={NAV} current="dashboard" account={account} {...props}>
      <PageHeader
        title="Overview"
        sub="Tuesday · 2 July 2026 · all systems nominal"
        actions={<><Button variant="secondary">Export</Button><Button variant="primary">+ New mailbox</Button></>}
      />

      {/* KPI row */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(140px,1fr))', gap: 16, marginBottom: 16 }}>
        {KPIS.map(k => (
          <StatCard key={k.label} size="lg" label={k.label} value={k.value}
            delta={k.delta} deltaTone={k.tone} dot={k.dot} pct={k.pct} note={k.note}
            icon={k.label === 'Storage' ? <Logo wordmark={false} markSize={15} color="var(--accent)" /> : undefined} />
        ))}
      </div>

      {/* Health + volume */}
      <div style={{ display: 'grid', gridTemplateColumns: '1.5fr 1fr', gap: 16, marginBottom: 16 }}>
        <Card pad>
          <div className="mf-row" style={{ marginBottom: 6 }}>
            <span className="mf-card__title">System health</span>
            <span className="mf-spacer mf-row mf-u-muted" style={{ gap: 7, fontSize: 12 }}><span className="mf-dot" />8 of 9 healthy</span>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0 24px' }}>
            {SERVICES.map(s => (
              <div key={s.name} className="mf-row" style={{ gap: 9, padding: '7px 0', borderTop: '1px solid var(--hair-soft)' }}>
                <span className={'mf-dot' + (s.tone !== 'green' ? ' mf-dot--' + s.tone : '')} />
                <span className="mf-u-mono" style={{ fontSize: 12.5, color: 'var(--ink)' }}>{s.name}</span>
                <span className="mf-spacer mf-u-mono" style={{ fontSize: 11.5, color: s.tone === 'amber' ? 'var(--amber)' : 'var(--faint)' }}>{s.meta}</span>
              </div>
            ))}
          </div>
        </Card>

        <Card pad style={{ display: 'flex', flexDirection: 'column' }}>
          <div className="mf-row"><span className="mf-card__title">Mail volume</span><span className="mf-spacer mf-u-faint" style={{ fontSize: 11.5 }}>7 days</span></div>
          <div style={{ fontFamily: 'var(--font-serif)', fontSize: 24, color: 'var(--ink-strong)', margin: '6px 0 12px' }}>7,420 <span style={{ fontSize: 12, fontFamily: 'var(--font-sans)', color: 'var(--muted)' }}>processed</span></div>
          <div className="mf-chart" style={{ marginTop: 'auto' }}>
            {CHART.map((h, i) => <div key={i} className={'mf-chart__bar' + (i === CHART.length - 1 ? ' mf-chart__bar--peak' : '')} style={{ height: h + '%' }} />)}
          </div>
          <div className="mf-row" style={{ justifyContent: 'space-between', marginTop: 8, fontSize: 10.5, color: 'var(--faint)' }}>
            {CHART_DAYS.map((d, i) => <span key={i}>{d}</span>)}
          </div>
        </Card>
      </div>

      {/* Queue preview */}
      <Card flush>
        <div className="mf-row" style={{ padding: '13px 16px 11px' }}>
          <span className="mf-card__title">Mail queue</span>
          <span className="mf-kbd" style={{ marginLeft: 8 }}>14</span>
          <a className="mf-spacer mf-u-accent" style={{ fontSize: 12.5, fontWeight: 500, cursor: 'pointer' }}>Manage queue →</a>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: '1.4fr 1.7fr .7fr .5fr .9fr', gap: 12, padding: '8px 16px', fontSize: 10.5, letterSpacing: '.05em', textTransform: 'uppercase', color: 'var(--faint)', fontWeight: 600, borderTop: '1px solid var(--hair-soft)' }}>
          <span>Sender</span><span>Recipient</span><span>Size</span><span>Age</span><span>Status</span>
        </div>
        {QUEUE.slice(0, 4).map((q, i) => (
          <div key={i} style={{ display: 'grid', gridTemplateColumns: '1.4fr 1.7fr .7fr .5fr .9fr', gap: 12, padding: '11px 16px', borderTop: '1px solid var(--hair-soft)', alignItems: 'center', fontSize: 13 }}>
            <span style={{ color: 'var(--ink)', fontWeight: 500 }}>{q.sender}</span>
            <span className="mf-u-muted">{q.rcpt}</span>
            <span className="mf-u-mono mf-u-muted" style={{ fontSize: 12.5 }}>{q.size}</span>
            <span className="mf-u-mono mf-u-faint" style={{ fontSize: 12.5 }}>{q.age}</span>
            <span><Pill tone={tone(q.status)}>{q.status}</Pill></span>
          </div>
        ))}
      </Card>
    </AppShell>
  );
}
