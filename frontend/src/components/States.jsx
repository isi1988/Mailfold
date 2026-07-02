import React from 'react';
import { Button } from '../ds/components/atoms/Button.jsx';
import { useT } from '../i18n/index.jsx';

// Shared loading / empty / error blocks so every page handles the three states
// consistently.
function StateBlock({ message, onRetry, children }) {
  const t = useT();
  return (
    <div style={{ padding: '56px 12px', textAlign: 'center' }} className="mf-u-muted">
      <div style={{ fontSize: 14 }}>{message}</div>
      {children}
      {onRetry && (
        <div style={{ marginTop: 14 }}>
          <Button variant="secondary" size="sm" onClick={onRetry}>{t('common.retry')}</Button>
        </div>
      )}
    </div>
  );
}

export function Loading({ message }) {
  const t = useT();
  return <StateBlock message={message || t('states.loading')} />;
}

export function Empty({ message, children }) {
  const t = useT();
  return <StateBlock message={message || t('states.empty')}>{children}</StateBlock>;
}

export function ErrorState({ error, onRetry }) {
  const t = useT();
  const msg = (error && error.message) || t('states.error');
  return <StateBlock message={msg} onRetry={onRetry} />;
}

// AsyncView renders the right state for a useApi() result, or children(data)
// when the data is ready.
export function AsyncView({ loading, error, reload, empty, children }) {
  if (loading) return <Loading />;
  if (error) return <ErrorState error={error} onRetry={reload} />;
  if (empty) return typeof empty === 'string' ? <Empty message={empty} /> : empty;
  return children;
}
