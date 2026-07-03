import React, { useState, useEffect, useCallback } from 'react';
import { Card } from '../ds/components/molecules/Card.jsx';
import { StatCard } from '../ds/components/molecules/StatCard.jsx';
import { Table, TableRow } from '../ds/components/organisms/Table.jsx';
import { Pill } from '../ds/components/atoms/Pill.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Logo } from '../ds/components/atoms/Logo.jsx';
import { api } from '../api/client.js';
import { useToast } from '../components/Toast.jsx';
import { human } from '../lib/format.js';
import { useT } from '../i18n/index.jsx';

const DNS_COLS = [
  { label: 'Type', w: '70px' },
  { label: 'Host', w: '1.3fr' },
  { label: 'Value', w: '2.4fr' },
  { label: 'Status', w: '92px' },
];
const STATUS_TONE = { ok: 'green', missing: 'amber', mismatch: 'red' };

const OK_ICON = <svg width="16" height="16" viewBox="0 0 16 16" fill="none" style={{ flex: 'none' }}><path d="M3 8.4l3 3 7-7" stroke="var(--green)" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" /></svg>;
const WARN_ICON = <svg width="16" height="16" viewBox="0 0 16 16" fill="none" style={{ flex: 'none' }}><path d="M8 2l6.5 11.5H1.5L8 2z" stroke="var(--amber)" strokeWidth="1.5" strokeLinejoin="round" /><path d="M8 6.5v3M8 11.6h.01" stroke="var(--amber)" strokeWidth="1.7" strokeLinecap="round" /></svg>;

/** Domain detail: stats, DKIM signing key, and a live DNS-records check. */
export function DomainDetailPage({ domain, onBack, onSettings }) {
  const t = useT();
  const { toast } = useToast();
  const name = domain.domain_name;
  const [dkim, setDkim] = useState(null);
  const [dns, setDns] = useState(null);
  const [checking, setChecking] = useState(false);
  const [busy, setBusy] = useState(false);

  const loadDkim = useCallback(async () => {
    try { setDkim(await api.get('/api/dkim/' + encodeURIComponent(name))); } catch { setDkim({}); }
  }, [name]);

  const checkDns = useCallback(async () => {
    setChecking(true);
    try { setDns(await api.get('/api/domains/' + encodeURIComponent(name) + '/dns')); }
    catch (e) { toast(t('domains.dns.failed'), (e && e.message) || ''); }
    finally { setChecking(false); }
  }, [name, t, toast]);

  useEffect(() => { loadDkim(); checkDns(); }, [loadDkim, checkDns]);

  const hasDkim = dkim && !Array.isArray(dkim) && (dkim.dkim_txt || dkim.pubkey);
  const dnsOk = !!(dns && dns.summary_ok);
  const used = Number(domain.bytes_total) || 0;
  const max = Number(domain.max_quota_for_domain) || 0;

  function copy(text) {
    try { navigator.clipboard.writeText(text); toast(t('domains.dkim.copied')); } catch { /* clipboard unavailable */ }
  }

  async function rotate() {
    if (busy) return;
    if (hasDkim && !window.confirm(t('domains.dkim.rotate') + ' — ' + name + '?')) return;
    setBusy(true);
    try {
      if (hasDkim) await api.del('/api/dkim', { items: [name] });
      await api.post('/api/dkim', { domains: name, dkim_selector: 'dkim', key_size: '2048' });
      toast(t('domains.dkim.generated', { domain: name }));
      await loadDkim();
      checkDns();
    } catch (e) {
      toast(t('domains.dkim.failed'), (e && e.body && e.body.message) || (e && e.message) || '');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div>
      <div onClick={onBack} className="mf-row" style={{ display: 'inline-flex', gap: 6, fontSize: 12.5, color: 'var(--muted)', cursor: 'pointer', marginBottom: 16 }}>
        <Icon name="chevron-left" size={14} /> {t('domains.title')}
      </div>

      <div className="mf-row" style={{ gap: 14, marginBottom: 22, alignItems: 'center' }}>
        <div className="mf-avatar mf-avatar--square" style={{ width: 46, height: 46, borderRadius: 12, flex: 'none' }}>
          <Logo wordmark={false} markSize={24} color="var(--accent-ink)" />
        </div>
        <div>
          <h1 style={{ fontFamily: 'var(--font-serif)', fontSize: 28, fontWeight: 600, color: 'var(--ink-strong)', margin: 0 }}>{name}</h1>
          <div className="mf-row" style={{ gap: 8, marginTop: 7 }}>
            <Pill tone={hasDkim ? 'green' : 'amber'}>{hasDkim ? t('domains.dns.dkimActive') : t('domains.dns.dkimMissing')}</Pill>
            <Pill tone={dnsOk ? 'green' : 'amber'}>{dnsOk ? t('domains.dns.verified') : t('domains.dns.pending')}</Pill>
          </div>
        </div>
        <div className="mf-spacer mf-row" style={{ gap: 10 }}>
          <Button variant="secondary" onClick={onSettings}>{t('domains.dns.settings')}</Button>
          <Button variant="primary" onClick={checkDns} disabled={checking}>{checking ? t('domains.dns.verifying') : t('domains.dns.verify')}</Button>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(150px,1fr))', gap: 14, marginBottom: 16 }}>
        <StatCard size="sm" label={t('domains.col.mailboxes')} value={domain.mboxes_in_domain ?? 0} />
        <StatCard size="sm" label={t('domains.col.aliases')} value={domain.aliases_in_domain ?? 0} />
        <StatCard size="sm" label={t('domains.col.storage')} value={human(used) + (max > 0 ? ' / ' + human(max) : '')} />
      </div>

      <Card pad style={{ marginBottom: 16 }}>
        <div className="mf-row" style={{ marginBottom: 14, alignItems: 'center' }}>
          <div>
            <span className="mf-card__title">{t('domains.dkim.title')}</span>
            {hasDkim && <span className="mf-u-mono mf-u-muted" style={{ marginLeft: 10, fontSize: 12 }}>{(dkim.dkim_selector || 'dkim')}._domainkey</span>}
          </div>
          <Button className="mf-spacer" variant="secondary" size="sm" onClick={rotate} disabled={busy}>
            {hasDkim ? t('domains.dkim.rotate') : t('domains.dkim.generate')}
          </Button>
        </div>
        {hasDkim ? (
          <>
            <div style={{ position: 'relative', background: 'var(--surface-2)', border: '1px solid var(--hair)', borderRadius: 10, padding: '14px 44px 14px 16px', fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--muted)', wordBreak: 'break-all', lineHeight: 1.8 }}>
              {dkim.dkim_txt}
              <button className="mf-btn mf-btn--secondary mf-btn--sm" style={{ position: 'absolute', top: 10, right: 10 }} onClick={() => copy(dkim.dkim_txt)}>{t('domains.dkim.copy')}</button>
            </div>
            {dkim.length ? <div style={{ marginTop: 10, fontSize: 12, color: 'var(--faint)' }}>{dkim.length}-bit RSA</div> : null}
          </>
        ) : (
          <div className="mf-u-muted" style={{ fontSize: 13 }}>{t('domains.dkim.none')}</div>
        )}
      </Card>

      <Card style={{ overflow: 'hidden' }}>
        <div className="mf-row" style={{ padding: '15px 18px 13px', alignItems: 'center' }}>
          <span className="mf-card__title">{t('domains.dns.title')}</span>
          <span className="mf-spacer" onClick={checking ? undefined : checkDns} style={{ fontSize: 12.5, color: 'var(--accent-ink)', cursor: checking ? 'default' : 'pointer', fontWeight: 500 }}>
            {checking ? t('domains.dns.verifying') : t('domains.dns.recheck')}
          </span>
        </div>
        {dns && (
          <div style={{ margin: '0 18px 6px', display: 'flex', alignItems: 'center', gap: 9, padding: '10px 13px', borderRadius: 10, background: dnsOk ? 'var(--green-soft)' : 'var(--amber-soft)' }}>
            {dnsOk ? OK_ICON : WARN_ICON}
            <span style={{ fontSize: 12.5, fontWeight: 600, color: dnsOk ? 'var(--green)' : 'var(--amber)' }}>{dns.summary}</span>
          </div>
        )}
        <Table columns={DNS_COLS}>
          {(dns ? dns.records : []).map((r, i) => (
            <TableRow plain key={i}>
              <span style={{ font: '600 11px var(--font-mono)', color: 'var(--ink)', background: 'var(--hair-soft)', padding: '3px 8px', borderRadius: 6, justifySelf: 'start' }}>{r.type}</span>
              <span className="mf-u-mono mf-u-muted mf-truncate" style={{ fontSize: 12.5 }}>{r.host}</span>
              <div style={{ minWidth: 0 }}>
                <span className="mf-u-mono mf-u-muted mf-truncate" style={{ display: 'block', fontSize: 12.5 }}>{r.value}</span>
                {r.status !== 'ok' && (
                  <div style={{ marginTop: 6, display: 'flex', alignItems: 'center', gap: 9, flexWrap: 'wrap' }}>
                    {r.found && <span style={{ font: '500 11.5px var(--font-mono)', color: 'var(--red)' }}>{t('domains.dns.found')}: {r.found}</span>}
                    <button className="mf-btn mf-btn--secondary mf-btn--sm" onClick={() => copy(r.value)}>{t('domains.dns.copyExpected')}</button>
                  </div>
                )}
              </div>
              <span><Pill tone={STATUS_TONE[r.status] || 'amber'}>{t('domains.dns.' + r.status)}</Pill></span>
            </TableRow>
          ))}
        </Table>
      </Card>
    </div>
  );
}
