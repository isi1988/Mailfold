import React from 'react';
import { Button } from '../ds/components/atoms/Button.jsx';

// Dark, high-contrast one-time reveal box for 2FA recovery codes — the same
// treatment as the API-key one-time token reveal, since both are "copy this now,
// it will never be shown again" secrets.
export function RecoveryCodesBox({ codes, copyLabel }) {
  async function copy() {
    try {
      await navigator.clipboard.writeText(codes.join('\n'));
    } catch {
      /* clipboard may be unavailable; the codes are still visible to copy by hand */
    }
  }
  return (
    <div style={{ position: 'relative', background: '#26221D', border: '1px solid #322C25', borderRadius: 10, padding: '14px 88px 14px 16px', font: '500 12.5px var(--font-mono)', color: '#E9E2D4', lineHeight: 1.9 }}>
      {codes.map(c => <div key={c}>{c}</div>)}
      <Button variant="primary" size="sm" onClick={copy} style={{ position: 'absolute', top: 10, right: 10 }}>{copyLabel}</Button>
    </div>
  );
}
