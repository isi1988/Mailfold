import React, { useState, useEffect } from 'react';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { Card } from '../ds/components/molecules/Card.jsx';
import { Chip } from '../ds/components/atoms/Chip.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { IconButton } from '../ds/components/atoms/IconButton.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { ConfirmModal } from '../ds/components/organisms/ConfirmModal.jsx';
import { useApi } from '../lib/useApi.js';
import { api } from '../api/client.js';
import { AsyncView } from '../components/States.jsx';
import { useToast } from '../components/Toast.jsx';
import { asList } from '../lib/format.js';
import { useT } from '../i18n/index.jsx';

// mailcow policy rows carry a numeric prefid plus the matched sender/pattern.
// The display value can live under `object`, `value`, or `object_from` depending
// on the mailcow version, so read the first that is present.
function entryValue(e) {
  return e.object || e.value || e.object_from || '';
}

// PolicyList renders one allow- or block-list card: a wrapping row of removable
// chips plus an inline "add entry" input. Wiring only touches /api/policy.
function PolicyList({ kind, tone, domain, data, loading, error, reload, onAdd, onRemove }) {
  const t = useT();
  const [pattern, setPattern] = useState('');
  const [busy, setBusy] = useState(false);
  const rows = asList(data);

  async function submit(e) {
    e.preventDefault();
    const value = pattern.trim();
    if (!value || busy) return;
    setBusy(true);
    try {
      await onAdd(value);
      setPattern('');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card pad>
      <div className="mf-row" style={{ marginBottom: 12 }}>
        <span className="mf-card__title">{t('spam.' + kind + '.title')}</span>
        <span className="mf-spacer mf-u-faint" style={{ fontSize: 12 }}>
          {t('spam.entryCount', { count: rows.length })}
        </span>
      </div>
      <div className="mf-u-muted" style={{ fontSize: 12.5, marginBottom: 14 }}>
        {t('spam.' + kind + '.hint')}
      </div>

      <AsyncView
        loading={loading}
        error={error}
        reload={reload}
        empty={rows.length === 0 ? t('spam.' + kind + '.empty') : null}
      >
        <div className="mf-row" style={{ flexWrap: 'wrap', gap: 7 }}>
          {rows.map((e, i) => {
            const value = entryValue(e);
            return (
              <Chip key={e.prefid ?? value + i} tone={tone} className="mf-row" style={{ gap: 6, alignItems: 'center' }}>
                <span className="mf-u-mono">{value}</span>
                <IconButton
                  aria-label={t('spam.remove')}
                  title={t('spam.remove')}
                  style={{ width: 18, height: 18 }}
                  onClick={() => onRemove(e)}
                >
                  <Icon name="close-sm" size={12} />
                </IconButton>
              </Chip>
            );
          })}
        </div>
      </AsyncView>

      <form onSubmit={submit} className="mf-row" style={{ gap: 8, marginTop: 16 }}>
        <Input
          className="mf-spacer"
          mono
          placeholder={t('spam.addPlaceholder')}
          value={pattern}
          onChange={ev => setPattern(ev.target.value)}
          disabled={!domain}
        />
        <Button variant="secondary" size="sm" type="submit" disabled={!domain || busy || !pattern.trim()}>
          {busy ? t('spam.adding') : t('spam.add')}
        </Button>
      </form>
    </Card>
  );
}

export function SpamPage() {
  const t = useT();
  const { toast } = useToast();
  const { data: domainsData, loading: domainsLoading, error: domainsError, reload: reloadDomains } = useApi('/api/domains', []);
  const domains = asList(domainsData);

  const [domain, setDomain] = useState('');
  const [confirm, setConfirm] = useState(null); // { kind, entry }

  // Default the picker to the first domain once the list arrives.
  useEffect(() => {
    if (!domain && domains.length > 0) setDomain(domains[0].domain_name);
  }, [domains, domain]);

  const allow = useApi(domain ? '/api/policy/allow/' + encodeURIComponent(domain) : null, [domain]);
  const deny = useApi(domain ? '/api/policy/deny/' + encodeURIComponent(domain) : null, [domain]);

  async function addEntry(kind, value) {
    const type = kind === 'allow' ? 'whitelist' : 'blacklist';
    try {
      await api.post('/api/policy', { domain, object: value, type });
      toast(t('spam.form.added', { value }));
      if (kind === 'allow') allow.reload(); else deny.reload();
    } catch (err) {
      toast(t('spam.form.failed'), (err && err.body && err.body.message) || (err && err.message) || '');
      throw err;
    }
  }

  async function doRemove() {
    const c = confirm;
    setConfirm(null);
    if (!c) return;
    const prefid = c.entry.prefid;
    try {
      await api.del('/api/policy', { items: [String(prefid)] });
      toast(t('spam.form.removed', { value: entryValue(c.entry) }));
      if (c.kind === 'allow') allow.reload(); else deny.reload();
    } catch (err) {
      toast(t('spam.form.failed'), (err && err.body && err.body.message) || (err && err.message) || '');
    }
  }

  return (
    <>
      <PageHeader title={t('spam.title')} sub={t('spam.subtitle')} />

      <div className="mf-row" style={{ marginBottom: 16, gap: 10, alignItems: 'flex-end' }}>
        <FormField label={t('spam.domain')} style={{ width: 280 }}>
          <select
            className="mf-input"
            value={domain}
            onChange={e => setDomain(e.target.value)}
            disabled={domains.length === 0}
          >
            {domains.length === 0 && <option value="">{t('spam.noDomains')}</option>}
            {domains.map(d => (
              <option key={d.domain_name} value={d.domain_name}>{d.domain_name}</option>
            ))}
          </select>
        </FormField>
      </div>

      <AsyncView
        loading={domainsLoading}
        error={domainsError}
        reload={reloadDomains}
        empty={!domainsLoading && domains.length === 0 ? t('spam.noDomains') : null}
      >
        <div className="mf-spam-grid" style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(320px,1fr))', gap: 16 }}>
          <PolicyList
            kind="allow"
            tone="allow"
            domain={domain}
            data={allow.data}
            loading={allow.loading}
            error={allow.error}
            reload={allow.reload}
            onAdd={v => addEntry('allow', v)}
            onRemove={entry => setConfirm({ kind: 'allow', entry })}
          />
          <PolicyList
            kind="block"
            tone="block"
            domain={domain}
            data={deny.data}
            loading={deny.loading}
            error={deny.error}
            reload={deny.reload}
            onAdd={v => addEntry('block', v)}
            onRemove={entry => setConfirm({ kind: 'block', entry })}
          />
        </div>
      </AsyncView>

      {confirm && (
        <ConfirmModal
          title={t('spam.form.removeTitle')}
          msg={t('spam.form.removeMsg', { value: entryValue(confirm.entry) })}
          cta={t('spam.form.remove')}
          danger
          onCancel={() => setConfirm(null)}
          onConfirm={doRemove}
        />
      )}
    </>
  );
}
