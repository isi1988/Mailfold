import React, { useRef, useState } from 'react';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Pill } from '../ds/components/atoms/Pill.jsx';
import { api } from '../api/client.js';
import { useT } from '../i18n/index.jsx';

/**
 * Bulk-create mailboxes from a CSV file: pick a file, review a per-row result
 * once imported. Required columns: local_part, domain, password. Optional:
 * name, quota_gb, active.
 *   onClose  () => void
 *   onSaved  () => void — called once at least one row is created
 */
export function MailboxImportDrawer({ onClose, onSaved }) {
  const t = useT();
  const fileInput = useRef(null);
  const [fileName, setFileName] = useState('');
  const [csvText, setCsvText] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState(null); // { results, created, failed }

  function pickFile() {
    fileInput.current && fileInput.current.click();
  }

  function onFileChange(e) {
    const file = e.target.files && e.target.files[0];
    if (!file) return;
    setError('');
    setFileName(file.name);
    const reader = new FileReader();
    reader.onload = () => setCsvText(String(reader.result || ''));
    reader.onerror = () => setError(t('mailboxes.importForm.readFailed'));
    reader.readAsText(file);
  }

  async function submit() {
    if (busy || !csvText.trim()) {
      setError(t('mailboxes.importForm.noFile'));
      return;
    }
    setBusy(true);
    setError('');
    try {
      const res = await api.post('/api/mailboxes/bulk', { csv: csvText });
      setResult(res);
      if (res.created > 0) onSaved();
    } catch (err) {
      setError((err && err.message) || t('mailboxes.importForm.failed'));
    } finally {
      setBusy(false);
    }
  }

  if (result) {
    const footer = <Button variant="primary" className="mf-spacer" onClick={onClose}>{t('mailboxes.importForm.done')}</Button>;
    return (
      <Drawer title={t('mailboxes.importForm.resultsTitle')} footer={footer} onClose={onClose}>
        <div className="mf-u-faint" style={{ fontSize: 12.5, marginBottom: 14 }}>
          {t('mailboxes.importForm.summary', { created: result.created, failed: result.failed })}
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {(result.results || []).map(r => (
            <div key={r.row} className="mf-row mf-row--between" style={{ padding: '8px 10px', borderRadius: 8, background: 'var(--surface-2)', border: '1px solid var(--hair-soft)' }}>
              <span className="mf-u-mono mf-truncate" style={{ fontSize: 12.5 }}>{r.mailbox || t('mailboxes.importForm.rowN', { row: r.row })}</span>
              {r.ok
                ? <Pill tone="green">{t('mailboxes.importForm.created')}</Pill>
                : <span className="mf-u-red mf-truncate" style={{ fontSize: 12, maxWidth: 220, textAlign: 'right' }} title={r.error}>{r.error}</span>}
            </div>
          ))}
        </div>
      </Drawer>
    );
  }

  const footer = (
    <>
      <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
      <Button variant="primary" onClick={submit} disabled={busy || !csvText.trim()}>
        {busy ? t('mailboxes.importForm.importing') : t('mailboxes.importForm.import')}
      </Button>
    </>
  );
  return (
    <Drawer title={t('mailboxes.importForm.title')} subtitle={t('mailboxes.importForm.sub')} footer={footer} onClose={onClose}>
      <div className="mf-u-muted" style={{ fontSize: 12.5, marginBottom: 16, lineHeight: 1.6 }}>
        {t('mailboxes.importForm.hint')}
      </div>
      <div className="mf-u-mono mf-u-faint" style={{ fontSize: 12, marginBottom: 16, padding: '10px 12px', borderRadius: 8, background: 'var(--surface-2)' }}>
        local_part,domain,password,name,quota_gb,active
      </div>
      <input ref={fileInput} type="file" accept=".csv,text/csv" onChange={onFileChange} style={{ display: 'none' }} />
      <Button variant="secondary" onClick={pickFile}>
        {fileName || t('mailboxes.importForm.chooseFile')}
      </Button>
      {error && <div className="mf-form-error" style={{ marginTop: 14 }} role="alert">{error}</div>}
    </Drawer>
  );
}
