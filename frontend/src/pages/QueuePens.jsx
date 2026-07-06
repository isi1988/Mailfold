import React from 'react';
import { Card } from '../ds/components/molecules/Card.jsx';

// MAX_ICONS caps how many little envelopes a pen draws before it switches to
// a "+N" overflow label — a pen showing 40 tiny icons would just be visual
// noise, but the count itself is never hidden, only the icon-per-message
// literalism.
const MAX_ICONS = 8;

const PENS = [
  { key: 'ready', color: '--green' },
  { key: 'queued', color: '--amber' },
  { key: 'failed', color: '--red' },
];

// CowEnvelope is a small decorative icon: an envelope with two "cow spots",
// playing on mailcow/Mailfold's own branding — used purely as flavour on the
// Mail Queue page, never as the source of truth for counts (the table below
// still is).
function CowEnvelope({ color }) {
  return (
    <svg width="26" height="19" viewBox="0 0 26 19" aria-hidden="true">
      <rect x="1" y="1" width="24" height="17" rx="3" fill="var(--surface)" stroke={`var(${color})`} strokeWidth="1.3" />
      <path d="M2 3 L13 11 L24 3" fill="none" stroke={`var(${color})`} strokeWidth="1.3" strokeLinecap="round" />
      <ellipse cx="7.5" cy="13.5" rx="2" ry="1.4" fill={`var(${color})`} opacity="0.5" />
      <ellipse cx="17.5" cy="14.6" rx="1.4" ry="1" fill={`var(${color})`} opacity="0.35" />
    </svg>
  );
}

// QueuePens renders the mail queue's three broad buckets — ready to send,
// waiting/retrying, and stuck — as pens of cow-envelopes, matching the
// "Secure. Scalable. Cow-Managed." concept art this feature is styled after.
// It is decorative: the same counts are always also in the plain table and
// page subtitle below, so nothing here is the only place a number lives.
export function QueuePens({ counts, t }) {
  return (
    <Card style={{ padding: '14px 18px 16px', marginBottom: 14 }}>
      <div className="mf-row mf-row--between" style={{ marginBottom: 12 }}>
        <div className="mf-card__title" style={{ padding: 0 }}>{t('queue.pens.title')}</div>
        <div className="mf-u-faint" style={{ fontSize: 11.5, fontStyle: 'italic' }}>{t('queue.pens.tagline')}</div>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12 }}>
        {PENS.map(pen => {
          const count = counts[pen.key] || 0;
          const shown = Math.min(count, MAX_ICONS);
          return (
            <div
              key={pen.key}
              style={{
                border: `1.5px dashed var(${pen.color})`,
                borderRadius: 12,
                padding: '10px 12px',
                background: `color-mix(in srgb, var(${pen.color}) 6%, transparent)`,
                minHeight: 84,
              }}
            >
              <div className="mf-row mf-row--between" style={{ marginBottom: 8 }}>
                <span style={{ fontSize: 12, fontWeight: 600, color: `var(${pen.color})` }}>{t('queue.pens.' + pen.key)}</span>
                <span className="mf-u-mono" style={{ fontSize: 12, color: `var(${pen.color})` }}>{count}</span>
              </div>
              {count === 0 ? (
                <div className="mf-u-faint" style={{ fontSize: 11.5 }}>{t('queue.pens.empty')}</div>
              ) : (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, alignItems: 'center' }}>
                  {Array.from({ length: shown }).map((_, i) => <CowEnvelope key={i} color={pen.color} />)}
                  {count > MAX_ICONS && (
                    <span className="mf-u-faint mf-u-mono" style={{ fontSize: 11 }}>{'+' + (count - MAX_ICONS)}</span>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </Card>
  );
}
