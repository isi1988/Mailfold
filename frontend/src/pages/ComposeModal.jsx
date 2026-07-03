import React, { useState, useRef, useEffect } from 'react';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Token } from '../ds/components/atoms/Token.jsx';
import { wm } from '../api/webmail.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';

function errText(err, fallback) {
  return (err && err.body && err.body.error) || (err && err.body && err.body.message) || (err && err.message) || fallback;
}

// RecipientInput is a functional address chip field: type an address and press
// Enter, comma or space to add a token; Backspace on an empty input removes the
// last one.
function RecipientInput({ values, onChange, placeholder }) {
  const [text, setText] = useState('');
  function commit(raw) {
    const parts = raw.split(/[,\s]+/).map(s => s.trim()).filter(Boolean);
    if (parts.length) onChange(Array.from(new Set([...values, ...parts])));
  }
  return (
    <div className="mf-multiselect mf-multiselect--flat">
      <div className="mf-multiselect__box">
        {values.map(v => <Token key={v} label={v} onRemove={() => onChange(values.filter(x => x !== v))} />)}
        <input
          className="mf-multiselect__input"
          placeholder={values.length ? '' : placeholder}
          value={text}
          onChange={e => setText(e.target.value)}
          onKeyDown={e => {
            if ((e.key === 'Enter' || e.key === ',' || e.key === ' ') && text.trim()) { e.preventDefault(); commit(text); setText(''); }
            else if (e.key === 'Backspace' && !text && values.length) { onChange(values.slice(0, -1)); }
          }}
          onBlur={() => { if (text.trim()) { commit(text); setText(''); } }}
        />
      </div>
    </div>
  );
}

// A single formatting-toolbar button. Uses mousedown (preventing default) so the
// editor keeps its selection while the command runs.
function RtButton({ title, onRun, children }) {
  return (
    <div
      title={title}
      onMouseDown={e => { e.preventDefault(); onRun(); }}
      style={{ width: 30, height: 30, borderRadius: 7, display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer', color: 'var(--muted)' }}
    >
      {children}
    </div>
  );
}
const RtSep = () => <div style={{ width: 1, height: 18, background: 'var(--hair-soft)', margin: '0 5px' }} />;

/**
 * Compose slide-over with a rich-text body.
 *   initial   { to, cc, bcc, subject, text, html } — prefill for reply/forward
 *   onClose   () => void
 *   onSent    () => void
 */
export function ComposeModal({ initial = {}, onClose, onSent }) {
  const t = useT();
  const { toast } = useToast();
  const asArray = v => (Array.isArray(v) ? v : v ? [v] : []);
  const [to, setTo] = useState(asArray(initial.to));
  const [cc, setCc] = useState(asArray(initial.cc));
  const [bcc, setBcc] = useState(asArray(initial.bcc));
  const [ccShown, setCcShown] = useState(cc.length > 0 || bcc.length > 0);
  const [subject, setSubject] = useState(initial.subject || '');
  const [busy, setBusy] = useState(false);
  const editorRef = useRef(null);

  // Seed the editor with any prefilled body once, keeping the caret usable.
  useEffect(() => {
    const el = editorRef.current;
    if (!el) return;
    if (initial.html) el.innerHTML = initial.html;
    else if (initial.text) el.innerText = initial.text;
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  function exec(cmd, arg) {
    document.execCommand(cmd, false, arg);
    if (editorRef.current) editorRef.current.focus();
  }

  async function send() {
    if (busy) return;
    if (to.length === 0) { toast(t('webmail.composer.needRecipient')); return; }
    setBusy(true);
    const html = editorRef.current ? editorRef.current.innerHTML : '';
    const text = editorRef.current ? editorRef.current.innerText : '';
    try {
      await wm.send({ to, cc, bcc, subject, html, text });
      toast(t('webmail.sent'));
      onSent();
      onClose();
    } catch (err) {
      toast(t('webmail.sendFailed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  const rows = [{ key: 'to', label: t('webmail.composer.to'), values: to, set: setTo, ph: 'name@example.com', toggle: true }];
  if (ccShown) {
    rows.push({ key: 'cc', label: t('webmail.composer.cc'), values: cc, set: setCc, ph: t('webmail.composer.ccPlaceholder') });
    rows.push({ key: 'bcc', label: t('webmail.composer.bcc'), values: bcc, set: setBcc, ph: t('webmail.composer.bccPlaceholder') });
  }

  return (
    <div className="mf-overlay mf-overlay--center" onClick={onClose}>
      <div
        onClick={e => e.stopPropagation()}
        style={{ width: 'min(760px, 94vw)', maxHeight: '82vh', background: 'var(--surface)', border: '1px solid var(--hair)', borderRadius: 16, boxShadow: 'var(--shadow-modal)', overflow: 'hidden', display: 'flex', flexDirection: 'column' }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '16px 24px', borderBottom: '1px solid var(--hair-soft)', background: 'var(--surface-2)' }}>
          <div style={{ width: 32, height: 32, borderRadius: 9, background: 'var(--accent-soft)', display: 'flex', alignItems: 'center', justifyContent: 'center', flex: 'none' }}>
            <svg width="17" height="17" viewBox="0 0 20 20" fill="none"><path d="M2.5 6l7.5 5 7.5-5M3 4.5h14a1 1 0 0 1 1 1v9a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1v-9a1 1 0 0 1 1-1z" stroke="var(--accent-ink)" strokeWidth="1.4" strokeLinejoin="round" /></svg>
          </div>
          <div style={{ fontFamily: 'var(--font-serif)', fontSize: 22, fontWeight: 600, color: 'var(--ink-strong)', letterSpacing: '-.01em' }}>{t('webmail.composer.newMessage')}</div>
          <div onClick={onClose} title={t('common.close')} className="mf-spacer" style={{ display: 'flex', alignItems: 'center', gap: 6, marginLeft: 'auto', cursor: 'pointer', color: 'var(--faint)', padding: '7px 12px', borderRadius: 9, font: '500 12.5px system-ui' }}>
            <svg width="16" height="16" viewBox="0 0 20 20" fill="none"><path d="M5 5l10 10M15 5L5 15" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" /></svg>{t('common.close')}
          </div>
        </div>

        <div style={{ padding: '14px 24px 0', display: 'flex', flexDirection: 'column', overflow: 'auto', flex: 1 }}>
          {rows.map(r => (
            <div key={r.key} style={{ display: 'flex', alignItems: 'flex-start', gap: 10, borderBottom: '1px solid var(--hair-soft)', paddingBottom: 9, marginBottom: 3 }}>
              <span style={{ fontSize: 12.5, color: 'var(--faint)', width: 32, flex: 'none', paddingTop: 8 }}>{r.label}</span>
              <div style={{ flex: 1, minWidth: 0 }}>
                <RecipientInput values={r.values} onChange={r.set} placeholder={r.ph} />
              </div>
              {r.toggle && !ccShown && (
                <span onClick={() => setCcShown(true)} style={{ fontSize: 12, color: 'var(--faint)', cursor: 'pointer', flex: 'none', paddingTop: 8 }}>Cc · Bcc</span>
              )}
            </div>
          ))}

          <div style={{ display: 'flex', alignItems: 'center', gap: 10, borderBottom: '1px solid var(--hair-soft)', padding: '11px 0' }}>
            <span style={{ fontSize: 12.5, color: 'var(--faint)', width: 56 }}>{t('webmail.composer.subject')}</span>
            <input
              placeholder={t('webmail.composer.subject')}
              value={subject}
              onChange={e => setSubject(e.target.value)}
              style={{ flex: 1, border: 'none', background: 'transparent', outline: 'none', fontSize: 13.5, fontWeight: 500, color: 'var(--ink)' }}
            />
          </div>

          {/* Formatting toolbar */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 2, padding: '9px 0', borderBottom: '1px solid var(--hair-soft)', flexWrap: 'wrap' }}>
            <RtButton title={t('webmail.composer.heading')} onRun={() => exec('formatBlock', '<h3>')}><span style={{ font: '700 12.5px system-ui' }}>H</span></RtButton>
            <RtSep />
            <RtButton title={t('webmail.composer.bold')} onRun={() => exec('bold')}><span style={{ font: '700 14px system-ui' }}>B</span></RtButton>
            <RtButton title={t('webmail.composer.italic')} onRun={() => exec('italic')}><span style={{ font: 'italic 600 15px Georgia,serif' }}>I</span></RtButton>
            <RtButton title={t('webmail.composer.underline')} onRun={() => exec('underline')}><span style={{ font: '600 14px system-ui', textDecoration: 'underline' }}>U</span></RtButton>
            <RtButton title={t('webmail.composer.strike')} onRun={() => exec('strikeThrough')}><span style={{ font: '600 14px system-ui', textDecoration: 'line-through' }}>S</span></RtButton>
            <RtSep />
            <RtButton title={t('webmail.composer.bulletList')} onRun={() => exec('insertUnorderedList')}>
              <svg width="16" height="16" viewBox="0 0 20 20" fill="none"><circle cx="3.5" cy="5.5" r="1.3" fill="currentColor" /><circle cx="3.5" cy="10" r="1.3" fill="currentColor" /><circle cx="3.5" cy="14.5" r="1.3" fill="currentColor" /><path d="M7.5 5.5h9M7.5 10h9M7.5 14.5h9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" /></svg>
            </RtButton>
            <RtButton title={t('webmail.composer.numberList')} onRun={() => exec('insertOrderedList')}>
              <svg width="16" height="16" viewBox="0 0 20 20" fill="none"><path d="M8 5.5h9M8 10h9M8 14.5h9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" /><path d="M2.4 4.2h1v3.4M2.2 7.6h2M2.2 12h2l-2 2.6h2" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round" strokeLinejoin="round" /></svg>
            </RtButton>
            <RtButton title={t('webmail.composer.quote')} onRun={() => exec('formatBlock', '<blockquote>')}>
              <svg width="16" height="16" viewBox="0 0 20 20" fill="none"><path d="M4 5.5h4v4c0 2-1.2 3.4-3.4 4.2M12 5.5h4v4c0 2-1.2 3.4-3.4 4.2" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" /></svg>
            </RtButton>
            <RtSep />
            <RtButton title={t('webmail.composer.link')} onRun={() => { const url = window.prompt(t('webmail.composer.linkPrompt')); if (url) exec('createLink', url); }}>
              <svg width="16" height="16" viewBox="0 0 20 20" fill="none"><path d="M8 12l4-4M7.5 13.5l-1.8 1.8a3 3 0 0 1-4.2-4.2l3.3-3.3M12.5 6.5l1.8-1.8a3 3 0 0 1 4.2 4.2l-3.3 3.3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" /></svg>
            </RtButton>
            <RtButton title={t('webmail.composer.clearFormat')} onRun={() => exec('removeFormat')}>
              <svg width="16" height="16" viewBox="0 0 20 20" fill="none"><path d="M7 5h9M10.5 5l-2 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" /><path d="M4 16h6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" /><path d="M13.5 12.5l3.5 3.5M17 12.5l-3.5 3.5" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" /></svg>
            </RtButton>
          </div>

          <div
            ref={editorRef}
            className="mf-rt-editor"
            contentEditable
            suppressContentEditableWarning
            data-placeholder={t('webmail.composer.bodyPlaceholder')}
            style={{ minHeight: 220, outline: 'none', font: '400 15px/1.7 var(--font-sans)', color: 'var(--ink)', padding: '14px 0', flex: 1, overflow: 'auto' }}
          />
        </div>

        <div className="mf-drawer__foot">
          <Button variant="primary" onClick={send} disabled={busy}>{busy ? t('webmail.sending') : t('webmail.send')}</Button>
          <Button variant="link" className="mf-spacer" onClick={onClose}>{t('webmail.composer.discard')}</Button>
        </div>
      </div>
    </div>
  );
}
