import React from 'react';
import { ACCOUNT, NAV, SPAM_KPIS, ALLOWLIST, BLOCKLIST, SPAM_RULES, initials } from '../data/sample.js';
import { AppShell } from '../components/organisms/AppShell.jsx';
import { PageHeader } from '../components/molecules/PageHeader.jsx';
import { StatCard } from '../components/molecules/StatCard.jsx';
import { Card } from '../components/molecules/Card.jsx';
import { ToggleRow } from '../components/molecules/ToggleRow.jsx';
import { Chip } from '../components/atoms/Chip.jsx';
import { Button } from '../components/atoms/Button.jsx';

const account = { ...ACCOUNT, initials: initials(ACCOUNT.name) };

export function Spam(props) {
  return (
    <AppShell nav={NAV} current="spam" account={account} {...props}>
      <PageHeader
        title="Spam filter"
        sub="Rspamd · scanning 128 mailboxes · Bayes trained on 8.9k messages"
        actions={<><Button variant="secondary">Reset</Button><Button variant="primary">Save changes</Button></>}
      />

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(140px,1fr))', gap: 14, marginBottom: 16 }}>
        {SPAM_KPIS.map(k => <StatCard key={k.label} size="sm" label={k.label} value={k.value} valueTone={k.tone === 'ink' ? undefined : k.tone} />)}
      </div>

      {/* Score thresholds */}
      <Card pad style={{ marginBottom: 16 }}>
        <div className="mf-card__title">Score thresholds</div>
        <div className="mf-u-muted" style={{ fontSize: 12.5, marginTop: 3 }}>How Rspamd acts on a message based on its spam score.</div>
        <div style={{ position: 'relative', paddingTop: 26, marginTop: 16 }}>
          <div style={{ position: 'absolute', top: 0, left: '40%', transform: 'translateX(-50%)', font: '600 11px var(--font-mono)', color: 'var(--amber)' }}>6.0</div>
          <div style={{ position: 'absolute', top: 0, left: '66.7%', transform: 'translateX(-50%)', font: '600 11px var(--font-mono)', color: 'var(--red)' }}>10.0</div>
          <div className="mf-thresh" style={{ background: 'linear-gradient(90deg,var(--green-soft) 0%,var(--green-soft) 40%,var(--amber-soft) 40%,var(--amber-soft) 66.7%,var(--red-soft) 66.7%,var(--red-soft) 100%)' }}>
            <div className="mf-thresh__handle" style={{ left: '40%', border: '2px solid var(--amber)' }} />
            <div className="mf-thresh__handle" style={{ left: '66.7%', border: '2px solid var(--red)' }} />
          </div>
          <div className="mf-row" style={{ justifyContent: 'space-between', marginTop: 8, font: '400 10.5px var(--font-mono)', color: 'var(--faint)' }}>
            <span>0</span><span>5</span><span>10</span><span>15</span>
          </div>
        </div>
        <div className="mf-row" style={{ gap: 22, marginTop: 18, flexWrap: 'wrap' }}>
          <span className="mf-row mf-u-muted" style={{ gap: 8, fontSize: 12.5 }}><span style={{ width: 9, height: 9, borderRadius: 3, background: 'var(--green)' }} />Pass — deliver to inbox <span className="mf-u-faint">(0–6)</span></span>
          <span className="mf-row mf-u-muted" style={{ gap: 8, fontSize: 12.5 }}><span style={{ width: 9, height: 9, borderRadius: 3, background: 'var(--amber)' }} />Add header — tag as spam <span className="mf-u-faint">(6–10)</span></span>
          <span className="mf-row mf-u-muted" style={{ gap: 8, fontSize: 12.5 }}><span style={{ width: 9, height: 9, borderRadius: 3, background: 'var(--red)' }} />Reject — bounce message <span className="mf-u-faint">(10+)</span></span>
        </div>
      </Card>

      {/* Allow / block lists */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginBottom: 16 }}>
        <Card pad>
          <div className="mf-row" style={{ marginBottom: 12 }}><span className="mf-card__title">Allowlist</span><span className="mf-spacer mf-u-accent" style={{ fontSize: 12, fontWeight: 500, cursor: 'pointer' }}>+ Add</span></div>
          <div className="mf-row" style={{ flexWrap: 'wrap', gap: 7 }}>{ALLOWLIST.map(x => <Chip key={x} tone="allow">{x}</Chip>)}</div>
        </Card>
        <Card pad>
          <div className="mf-row" style={{ marginBottom: 12 }}><span className="mf-card__title">Blocklist</span><span className="mf-spacer mf-u-accent" style={{ fontSize: 12, fontWeight: 500, cursor: 'pointer' }}>+ Add</span></div>
          <div className="mf-row" style={{ flexWrap: 'wrap', gap: 7 }}>{BLOCKLIST.map(x => <Chip key={x} tone="block">{x}</Chip>)}</div>
        </Card>
      </div>

      {/* Rule toggles */}
      <Card style={{ padding: '6px 18px 14px' }}>
        {SPAM_RULES.map(r => <ToggleRow key={r.title} title={r.title} desc={r.desc} on={r.on} />)}
      </Card>
    </AppShell>
  );
}
