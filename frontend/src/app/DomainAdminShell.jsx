import React, { useEffect, useState } from 'react';
import { Logo } from '../ds/components/atoms/Logo.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { ConfirmModal } from '../ds/components/organisms/ConfirmModal.jsx';
import { useToast } from '../components/Toast.jsx';
import { useDomainAdminAuth } from '../auth/DomainAdminAuthContext.jsx';
import { domainAdminApi } from '../api/domainAdmin.js';
import { useT } from '../i18n/index.jsx';

function errText(err, fallback) {
  return (err && err.body && err.body.error) || (err && err.message) || fallback;
}

function ProviderForm({ initial, domains, busy, onSave, onCancel }) {
  const t = useT();
  const [name, setName] = useState((initial && initial.name) || '');
  const [issuer, setIssuer] = useState((initial && initial.issuer) || '');
  const [clientId, setClientId] = useState((initial && initial.client_id) || '');
  const [clientSecret, setClientSecret] = useState('');
  const [redirectURL, setRedirectURL] = useState((initial && initial.redirect_url) || (window.location.origin + '/api/auth/sso/callback'));
  const [selected, setSelected] = useState((initial && initial.domains) || domains);

  const toggleDomain = d => setSelected(cur => (cur.includes(d) ? cur.filter(x => x !== d) : [...cur, d]));

  return (
    <div style={{ border: '1px solid var(--hair)', borderRadius: 10, padding: 14, marginBottom: 12 }}>
      <FormField label={t('domainAdmin.sso.name')}>
        <Input value={name} onChange={e => setName(e.target.value)} placeholder={t('domainAdmin.sso.namePlaceholder')} />
      </FormField>
      <FormField label={t('domainAdmin.sso.issuer')}>
        <Input value={issuer} onChange={e => setIssuer(e.target.value)} placeholder="https://idp.example.com" />
      </FormField>
      <div className="mf-row" style={{ gap: 10 }}>
        <FormField label={t('domainAdmin.sso.clientId')} style={{ flex: 1 }}>
          <Input mono value={clientId} onChange={e => setClientId(e.target.value)} />
        </FormField>
        <FormField label={t('domainAdmin.sso.clientSecret')} style={{ flex: 1 }}>
          <Input mono type="password" value={clientSecret} onChange={e => setClientSecret(e.target.value)} placeholder={initial ? t('domainAdmin.sso.secretHint') : ''} />
        </FormField>
      </div>
      <FormField label={t('domainAdmin.sso.redirectUrl')}>
        <Input mono value={redirectURL} onChange={e => setRedirectURL(e.target.value)} />
      </FormField>
      <FormField label={t('domainAdmin.sso.domains')} hint={t('domainAdmin.sso.domainsHint')}>
        <div className="mf-input" style={{ padding: 8, display: 'flex', flexDirection: 'column', gap: 2 }}>
          {domains.map(d => (
            <label key={d} className="mf-row" style={{ gap: 8, padding: '4px 2px', cursor: 'pointer', fontSize: 13 }}>
              <input type="checkbox" checked={selected.includes(d)} onChange={() => toggleDomain(d)} />
              <span className="mf-u-mono">{d}</span>
            </label>
          ))}
        </div>
      </FormField>
      <div className="mf-row" style={{ gap: 8, marginTop: 12, justifyContent: 'flex-end' }}>
        <Button variant="secondary" size="sm" onClick={onCancel}>{t('common.cancel')}</Button>
        <Button
          variant="primary" size="sm" disabled={busy || !name.trim() || !issuer.trim() || !clientId.trim() || (!initial && !clientSecret.trim())}
          onClick={() => onSave({ name, issuer, client_id: clientId, client_secret: clientSecret, redirect_url: redirectURL, domains: selected })}
        >
          {busy ? t('common.saving') : t('common.save')}
        </Button>
      </div>
    </div>
  );
}

function SSOProvidersSection() {
  const t = useT();
  const { domains } = useDomainAdminAuth();
  const { toast } = useToast();
  const [providers, setProviders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState(null); // 'new' | provider | null
  const [busy, setBusy] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(null);

  async function reload() {
    const list = await domainAdminApi.ssoProviders.list();
    setProviders(list || []);
  }
  useEffect(() => { reload().catch(() => {}).finally(() => setLoading(false)); }, []);

  async function save(values) {
    setBusy(true);
    try {
      if (editing && editing !== 'new') await domainAdminApi.ssoProviders.update(editing.id, values);
      else await domainAdminApi.ssoProviders.create(values);
      toast(t('domainAdmin.sso.saved'));
      setEditing(null);
      await reload();
    } catch (err) {
      toast(t('domainAdmin.sso.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  async function doDelete() {
    const p = confirmDelete;
    setConfirmDelete(null);
    try {
      await domainAdminApi.ssoProviders.del(p.id);
      toast(t('domainAdmin.sso.deleted'));
      await reload();
    } catch (err) {
      toast(t('domainAdmin.sso.failed'), errText(err, ''));
    }
  }

  if (loading) return <div className="mf-u-muted" style={{ fontSize: 13, padding: '10px 0' }}>{t('common.loading')}</div>;

  return (
    <>
      <div className="mf-u-faint" style={{ fontSize: 12.5, marginBottom: 14, lineHeight: 1.5 }}>{t('domainAdmin.sso.hint')}</div>
      {providers.length === 0 && editing !== 'new' && (
        <div className="mf-u-muted" style={{ fontSize: 13, padding: '8px 0 14px' }}>{t('domainAdmin.sso.empty')}</div>
      )}
      {providers.map(p => (
        editing === p ? (
          <ProviderForm key={p.id} initial={p} domains={domains} busy={busy} onSave={save} onCancel={() => setEditing(null)} />
        ) : (
          <div key={p.id} className="mf-row" style={{ gap: 10, padding: '10px 4px', borderTop: '1px solid var(--hair-soft)' }}>
            <div className="mf-min0" style={{ flex: 1, cursor: p.editable ? 'pointer' : 'default' }} onClick={() => p.editable && setEditing(p)}>
              <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>{p.name}</div>
              <div className="mf-u-faint" style={{ fontSize: 12 }}>
                {p.all_domains ? t('domainAdmin.sso.allDomains') : (p.domains || []).join(', ')}
                {!p.editable && ' · ' + t('domainAdmin.sso.shared')}
              </div>
            </div>
            {p.editable && (
              <Button variant="ghost" size="sm" title={t('common.delete')} onClick={() => setConfirmDelete(p)}>
                <Icon name="trash" size={15} />
              </Button>
            )}
          </div>
        )
      ))}
      {editing === 'new' ? (
        <ProviderForm domains={domains} busy={busy} onSave={save} onCancel={() => setEditing(null)} />
      ) : (
        <Button variant="secondary" size="sm" onClick={() => setEditing('new')} style={{ marginTop: 12 }}>{t('domainAdmin.sso.add')}</Button>
      )}
      {confirmDelete && (
        <ConfirmModal
          title={t('domainAdmin.sso.deleteTitle')}
          msg={t('domainAdmin.sso.deleteMsg', { name: confirmDelete.name })}
          cta={t('common.delete')}
          danger
          onCancel={() => setConfirmDelete(null)}
          onConfirm={doDelete}
        />
      )}
    </>
  );
}

export function DomainAdminShell() {
  const t = useT();
  const { user, domains, logout } = useDomainAdminAuth();
  const [confirmLogout, setConfirmLogout] = useState(false);
  return (
    <div style={{ height: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--bg)' }}>
      <header className="mf-row" style={{ padding: '12px 22px', borderBottom: '1px solid var(--hair)', flex: 'none', gap: 12 }}>
        <Logo size="sm" />
        <span className="mf-spacer" />
        <span className="mf-u-faint" style={{ fontSize: 12.5 }}>{t('domainAdmin.signedInAs', { user })}</span>
        <Button variant="link" size="sm" onClick={() => setConfirmLogout(true)}>{t('domainAdmin.signOut')}</Button>
      </header>
      <div style={{ flex: 1, minHeight: 0, overflow: 'auto', padding: '28px 22px' }}>
        <div style={{ maxWidth: 640, margin: '0 auto' }}>
          <div style={{ fontFamily: 'var(--font-serif)', fontSize: 22, fontWeight: 600, color: 'var(--ink-strong)', marginBottom: 4 }}>
            {t('domainAdmin.title')}
          </div>
          <div className="mf-u-faint" style={{ fontSize: 13, marginBottom: 20 }}>
            {t('domainAdmin.sub', { domains: domains.join(', ') })}
          </div>
          <SSOProvidersSection />
        </div>
      </div>
      {confirmLogout && (
        <ConfirmModal
          title={t('domainAdmin.signOutConfirm.title')}
          msg={t('domainAdmin.signOutConfirm.msg')}
          cta={t('domainAdmin.signOut')}
          onCancel={() => setConfirmLogout(false)}
          onConfirm={logout}
        />
      )}
    </div>
  );
}
