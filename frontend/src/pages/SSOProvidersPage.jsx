import React, { useEffect, useState } from 'react';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Toggle } from '../ds/components/atoms/Toggle.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { Table, TableRow } from '../ds/components/organisms/Table.jsx';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { ConfirmModal } from '../ds/components/organisms/ConfirmModal.jsx';
import { AsyncView } from '../components/States.jsx';
import { useApi } from '../lib/useApi.js';
import { asList, isActive } from '../lib/format.js';
import { decodeIdnDomain } from '../lib/idn.js';
import { api } from '../api/client.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';

function errText(err, fallback) {
  return (err && err.body && err.body.error) || (err && err.message) || fallback;
}

function ProviderDrawer({ provider, allDomains, onClose, onSaved }) {
  const t = useT();
  const { toast } = useToast();
  const editing = !!provider;
  const [name, setName] = useState((provider && provider.name) || '');
  const [issuer, setIssuer] = useState((provider && provider.issuer) || '');
  const [clientId, setClientId] = useState((provider && provider.client_id) || '');
  const [clientSecret, setClientSecret] = useState('');
  const [redirectURL, setRedirectURL] = useState((provider && provider.redirect_url) || (window.location.origin + '/api/auth/sso/callback'));
  const [scopeAll, setScopeAll] = useState(provider ? provider.all_domains : true);
  const [selectedDomains, setSelectedDomains] = useState((provider && provider.domains) || []);
  const [active, setActive] = useState(provider ? provider.active : true);
  const [busy, setBusy] = useState(false);

  const toggleDomain = d => setSelectedDomains(cur => (cur.includes(d) ? cur.filter(x => x !== d) : [...cur, d]));

  async function save() {
    if (busy) return;
    setBusy(true);
    const body = {
      name, issuer, client_id: clientId, client_secret: clientSecret, redirect_url: redirectURL,
      all_domains: scopeAll, domains: scopeAll ? [] : selectedDomains, active,
    };
    try {
      if (editing) await api.put('/api/sso-providers', { id: provider.id, ...body });
      else await api.post('/api/sso-providers', body);
      toast(editing ? t('sso.updated') : t('sso.created'));
      onSaved();
      onClose();
    } catch (err) {
      toast(t('sso.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  const footer = (
    <>
      <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
      <Button variant="primary" onClick={save} disabled={busy || !name.trim() || !issuer.trim() || !clientId.trim() || (!editing && !clientSecret.trim())}>
        {busy ? t('common.saving') : (editing ? t('common.save') : t('common.create'))}
      </Button>
    </>
  );

  return (
    <Drawer title={editing ? t('sso.editTitle') : t('sso.newTitle')} subtitle={t('sso.sub')} footer={footer} onClose={onClose} wide>
      <FormField label={t('sso.name')}>
        <Input value={name} onChange={e => setName(e.target.value)} placeholder={t('sso.namePlaceholder')} />
      </FormField>
      <FormField label={t('sso.issuer')}>
        <Input value={issuer} onChange={e => setIssuer(e.target.value)} placeholder="https://idp.example.com" />
      </FormField>
      <div className="mf-row" style={{ gap: 10 }}>
        <FormField label={t('sso.clientId')} style={{ flex: 1 }}>
          <Input mono value={clientId} onChange={e => setClientId(e.target.value)} />
        </FormField>
        <FormField label={t('sso.clientSecret')} style={{ flex: 1 }}>
          <Input mono type="password" value={clientSecret} onChange={e => setClientSecret(e.target.value)} placeholder={editing ? t('sso.secretHint') : ''} />
        </FormField>
      </div>
      <FormField label={t('sso.redirectUrl')}>
        <Input mono value={redirectURL} onChange={e => setRedirectURL(e.target.value)} />
      </FormField>
      <div className="mf-row mf-row--between" style={{ marginTop: 8 }}>
        <span className="mf-u-muted" style={{ fontSize: 13 }}>{t('sso.allDomains')}</span>
        <Toggle on={scopeAll} onClick={() => setScopeAll(v => !v)} style={{ cursor: 'pointer' }} />
      </div>
      {!scopeAll && (
        <FormField label={t('sso.domains')}>
          <div className="mf-input" style={{ padding: 8, maxHeight: 180, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 2 }}>
            {allDomains.length === 0 ? (
              <span className="mf-u-faint" style={{ fontSize: 12.5 }}>—</span>
            ) : allDomains.map(d => (
              <label key={d} className="mf-row" style={{ gap: 8, padding: '4px 2px', cursor: 'pointer', fontSize: 13 }}>
                <input type="checkbox" checked={selectedDomains.includes(d)} onChange={() => toggleDomain(d)} />
                <span className="mf-u-mono">{decodeIdnDomain(d)}</span>
              </label>
            ))}
          </div>
        </FormField>
      )}
      <div className="mf-row mf-row--between" style={{ marginTop: 8 }}>
        <span className="mf-u-muted" style={{ fontSize: 13 }}>{t('common.active')}</span>
        <Toggle on={active} onClick={() => setActive(v => !v)} style={{ cursor: 'pointer' }} />
      </div>
    </Drawer>
  );
}

export function SSOProvidersPage() {
  const t = useT();
  const { toast } = useToast();
  const { data, loading, error, reload } = useApi('/api/sso-providers', []);
  const domainsApi = useApi('/api/domains', []);
  const allDomains = asList(domainsApi.data).map(d => d.domain_name);
  const [drawer, setDrawer] = useState(null); // { mode, provider }
  const [confirmDelete, setConfirmDelete] = useState(null);

  const rows = asList(data);

  async function doDelete() {
    const p = confirmDelete;
    setConfirmDelete(null);
    try {
      await api.del('/api/sso-providers', { id: String(p.id) });
      toast(t('sso.deleted'));
      reload();
    } catch (err) {
      toast(t('sso.failed'), errText(err, ''));
    }
  }

  return (
    <>
      <PageHeader
        title={t('sso.title')}
        sub={t('sso.sub')}
        actions={<Button variant="primary" onClick={() => setDrawer({ mode: 'create' })}>{t('sso.add')}</Button>}
      />
      <AsyncView loading={loading || domainsApi.loading} error={error} reload={reload} empty={rows.length === 0 ? t('sso.empty') : null}>
        <Table columns={[
          { label: t('sso.col.name'), w: '1.5fr' },
          { label: t('sso.col.scope'), w: '2fr' },
          { label: t('sso.col.createdBy'), w: '1fr' },
          { label: t('sso.col.status'), w: '.8fr' },
          { label: '', w: '18px' },
        ]}>
          {rows.map(p => (
            <TableRow key={p.id} onClick={() => setDrawer({ mode: 'edit', provider: p })} style={{ cursor: 'pointer' }}>
              <span className="mf-truncate" style={{ fontSize: 13, color: 'var(--ink)' }}>{p.name}</span>
              <span className="mf-u-faint mf-truncate" style={{ fontSize: 12.5 }}>
                {p.all_domains ? t('sso.allDomains') : (p.domains || []).map(decodeIdnDomain).join(', ')}
              </span>
              <span className="mf-u-faint" style={{ fontSize: 12.5 }}>{p.created_by || t('sso.superAdmin')}</span>
              <span className="mf-u-faint" style={{ fontSize: 12.5 }}>{isActive(p.active) ? t('common.active') : t('common.disabled')}</span>
              <Button variant="ghost" size="sm" title={t('common.delete')} onClick={e => { e.stopPropagation(); setConfirmDelete(p); }}>
                <Icon name="trash" size={15} />
              </Button>
            </TableRow>
          ))}
        </Table>
      </AsyncView>
      {drawer && (
        <ProviderDrawer
          provider={drawer.mode === 'edit' ? drawer.provider : null}
          allDomains={allDomains}
          onClose={() => setDrawer(null)}
          onSaved={reload}
        />
      )}
      {confirmDelete && (
        <ConfirmModal
          title={t('sso.deleteTitle')}
          msg={t('sso.deleteMsg', { name: confirmDelete.name })}
          cta={t('common.delete')}
          danger
          onCancel={() => setConfirmDelete(null)}
          onConfirm={doDelete}
        />
      )}
    </>
  );
}
