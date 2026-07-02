import React, { useState } from 'react';
import { Input } from '../ds/components/atoms/Input.jsx';

const EyeOn = ({ size = 17 }) => (
  <svg width={size} height={size} viewBox="0 0 20 20" fill="none" aria-hidden="true">
    <path d="M1.6 10S4.6 4.6 10 4.6 18.4 10 18.4 10 15.4 15.4 10 15.4 1.6 10 1.6 10z" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" />
    <circle cx="10" cy="10" r="2.5" stroke="currentColor" strokeWidth="1.4" />
  </svg>
);
const EyeOff = ({ size = 17 }) => (
  <svg width={size} height={size} viewBox="0 0 20 20" fill="none" aria-hidden="true">
    <path d="M1.6 10S4.6 4.6 10 4.6c1.5 0 2.8.4 4 1M18.4 10s-1 1.9-3 3.4M12 12a2.5 2.5 0 01-4-3" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" />
    <path d="M3.5 3.5l13 13" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
  </svg>
);

// Password input with a show/hide "eye" toggle.
export function PasswordField({ value, onChange, size = 'lg', autoComplete = 'current-password', placeholder = '••••••••', ...rest }) {
  const [show, setShow] = useState(false);
  return (
    <div style={{ position: 'relative' }}>
      <Input
        type={show ? 'text' : 'password'}
        size={size}
        value={value}
        onChange={onChange}
        autoComplete={autoComplete}
        placeholder={placeholder}
        style={{ paddingRight: 42 }}
        {...rest}
      />
      <button
        type="button"
        tabIndex={-1}
        onClick={() => setShow(s => !s)}
        aria-label={show ? 'Hide password' : 'Show password'}
        aria-pressed={show}
        style={{ position: 'absolute', right: 8, top: '50%', transform: 'translateY(-50%)', background: 'none', border: 'none', cursor: 'pointer', color: 'var(--faint)', padding: 6, display: 'flex', alignItems: 'center' }}
      >
        {show ? <EyeOff /> : <EyeOn />}
      </button>
    </div>
  );
}
