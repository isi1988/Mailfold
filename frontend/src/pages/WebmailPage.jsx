import React, { useState, useEffect, useCallback, useRef } from 'react';
import { FolderItem } from '../ds/components/molecules/FolderItem.jsx';
import { MailListItem } from '../ds/components/molecules/MailListItem.jsx';
import { SearchInput } from '../ds/components/molecules/SearchInput.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { PasswordField } from '../components/PasswordField.jsx';
import { Textarea } from '../ds/components/atoms/Textarea.jsx';
import { Label } from '../ds/components/atoms/Label.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { Avatar } from '../ds/components/atoms/Avatar.jsx';
import { initials } from '../ds/data/sample.js';
import { useWebmailAuth } from '../auth/WebmailAuthContext.jsx';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';
import { wm, downloadAttachment, subscribeMail } from '../api/webmail.js';
import { Loading, ErrorState, Empty } from '../components/States.jsx';

const FOLDER_ICON = { inbox: 'inbox', sent: 'send', drafts: 'drafts', archive: 'archive', junk: 'shield', spam: 'shield', trash: 'trash' };

const hasFlag = (flags, f) => Array.isArray(flags) && flags.includes(f);
const addrLabel = list => {
  const a = Array.isArray(list) && list[0];
  return a ? (a.name || a.email || '') : '';
};
const shortTime = iso => {
  const d = new Date(iso);
  if (isNaN(d)) return '';
  const now = new Date();
  return d.toDateString() === now.toDateString()
    ? d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    : d.toLocaleDateString([], { month: 'short', day: 'numeric' });
};

// Inline mailbox login shown when there is no webmail session (e.g. an admin
// opening the Webmail page).
function WebmailLogin() {
  const { login } = useWebmailAuth();
  const t = useT();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit(e) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError('');
    try {
      await login(email.trim(), password);
    } catch (err) {
      setError(err && err.status === 401 ? t('webmail.invalid') : (err.message || t('webmail.invalid')));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div style={{ maxWidth: 380, margin: '8vh auto 0' }}>
      <form onSubmit={submit}>
        <div className="mf-login__title" style={{ fontSize: 24 }}>{t('webmail.signInTitle')}</div>
        <div className="mf-login__sub">{t('webmail.signInSub')}</div>
        <FormField label={t('webmail.mailbox')} style={{ marginTop: 22 }}>
          <Input size="lg" placeholder="you@example.com" autoComplete="username" value={email} onChange={e => setEmail(e.target.value)} />
        </FormField>
        <FormField label={t('webmail.password')}>
          <PasswordField value={password} onChange={e => setPassword(e.target.value)} />
        </FormField>
        {error && <div className="mf-u-danger" style={{ marginTop: 14, fontSize: 13 }} role="alert">{error}</div>}
        <Button variant="primary" block size="lg" type="submit" disabled={busy} style={{ marginTop: 22 }}>{busy ? t('webmail.signingIn') : t('webmail.signIn')}</Button>
      </form>
    </div>
  );
}

// Compose slide-over wired to POST /api/webmail/send. `initial` prefills the
// fields for a reply.
function ComposeDrawer({ onClose, onSent, initial = {} }) {
  const t = useT();
  const { toast } = useToast();
  const [to, setTo] = useState(initial.to || '');
  const [subject, setSubject] = useState(initial.subject || '');
  const [text, setText] = useState(initial.text || '');
  const [busy, setBusy] = useState(false);

  async function send() {
    if (busy) return;
    const recipients = to.split(',').map(s => s.trim()).filter(Boolean);
    if (recipients.length === 0) return;
    setBusy(true);
    try {
      await wm.send({ to: recipients, subject, text });
      toast(t('webmail.sent'));
      onSent();
      onClose();
    } catch (err) {
      toast(t('webmail.sendFailed'), (err && err.body && err.body.error) || (err && err.message) || '');
    } finally {
      setBusy(false);
    }
  }

  const footer = (
    <>
      <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
      <Button variant="primary" onClick={send} disabled={busy}>{busy ? t('webmail.sending') : t('webmail.send')}</Button>
    </>
  );

  return (
    <Drawer title={t('webmail.newMessage')} footer={footer} onClose={onClose} wide>
      <FormField label={t('webmail.to')}>
        <Input placeholder="name@example.com, other@example.com" value={to} onChange={e => setTo(e.target.value)} />
      </FormField>
      <FormField label={t('webmail.subject')}>
        <Input value={subject} onChange={e => setSubject(e.target.value)} />
      </FormField>
      <Label>{t('webmail.body')}</Label>
      <Textarea placeholder={t('webmail.body')} value={text} onChange={e => setText(e.target.value)} style={{ minHeight: 220 }} />
    </Drawer>
  );
}

function WebmailClient() {
  const t = useT();
  const { email, expire, logout } = useWebmailAuth();
  const { toast } = useToast();

  const [folders, setFolders] = useState([]);
  const [folder, setFolder] = useState('INBOX');
  const [messages, setMessages] = useState([]);
  const [selected, setSelected] = useState(null); // MessageHeader
  const [body, setBody] = useState(null);
  const [q, setQ] = useState('');
  const [loadingList, setLoadingList] = useState(true);
  const [error, setError] = useState(null);
  const [composing, setComposing] = useState(false);

  const onErr = useCallback(err => {
    if (err && err.status === 401) { expire(); return; }
    setError(err);
  }, [expire]);

  useEffect(() => {
    wm.folders().then(setFolders).catch(onErr);
  }, [onErr]);

  const loadMessages = useCallback(async f => {
    setLoadingList(true);
    setError(null);
    setSelected(null);
    setBody(null);
    try {
      const msgs = await wm.messages(f, 50);
      setMessages(Array.isArray(msgs) ? msgs : []);
    } catch (err) {
      onErr(err);
    } finally {
      setLoadingList(false);
    }
  }, [onErr]);

  useEffect(() => { loadMessages(folder); }, [folder, loadMessages]);

  // Live new-mail notifications over SSE. The handler is kept in a ref so the
  // stream is opened once (on mount) yet always runs the latest logic — it
  // toasts an alert and, when the INBOX is open, prepends the new headers
  // without disturbing an open message.
  const notifyRef = useRef(null);
  notifyRef.current = data => {
    const incoming = Array.isArray(data.messages) ? data.messages : [];
    const n = data.count || incoming.length;
    if (!n) return;
    const first = incoming[0];
    const who = first && first.from && first.from[0] ? (first.from[0].name || first.from[0].email) : '';
    toast(t('webmail.newMail', { count: n }), first ? [who, first.subject].filter(Boolean).join(' — ') : '');
    if (folder === 'INBOX' && incoming.length) {
      setMessages(list => {
        const known = new Set(list.map(m => m.uid));
        const fresh = incoming.filter(m => !known.has(m.uid));
        return fresh.length ? [...fresh, ...list] : list;
      });
    }
  };
  useEffect(() => {
    const unsub = subscribeMail(data => { if (notifyRef.current) notifyRef.current(data); });
    return unsub;
  }, []);

  async function openMessage(m) {
    setSelected(m);
    setBody(null);
    try {
      const full = await wm.message(folder, m.uid);
      setBody(full);
      if (!hasFlag(m.flags, '\\Seen')) {
        // The write API takes friendly flag names ('seen'); the flags array
        // returned by the read API uses raw IMAP flags ('\\Seen').
        await wm.flag(folder, m.uid, 'seen', true);
        setMessages(list => list.map(x => (x.uid === m.uid ? { ...x, flags: [...(x.flags || []), '\\Seen'] } : x)));
      }
    } catch (err) {
      onErr(err);
    }
  }

  async function toggleStar(m, e) {
    e.stopPropagation();
    const starred = hasFlag(m.flags, '\\Flagged');
    try {
      await wm.flag(folder, m.uid, 'flagged', !starred);
      setMessages(list => list.map(x => (x.uid === m.uid
        ? { ...x, flags: starred ? x.flags.filter(f => f !== '\\Flagged') : [...(x.flags || []), '\\Flagged'] }
        : x)));
    } catch (err) {
      onErr(err);
    }
  }

  async function del(m) {
    try {
      await wm.del(folder, m.uid);
      setMessages(list => list.filter(x => x.uid !== m.uid));
      if (selected && selected.uid === m.uid) { setSelected(null); setBody(null); }
      toast(t('webmail.deleted'));
    } catch (err) {
      toast(t('webmail.actionFailed'), (err && err.message) || '');
    }
  }

  async function archive(m) {
    try {
      await wm.move(folder, m.uid, 'Archive');
      setMessages(list => list.filter(x => x.uid !== m.uid));
      if (selected && selected.uid === m.uid) { setSelected(null); setBody(null); }
      toast(t('webmail.archived'));
    } catch (err) {
      toast(t('webmail.actionFailed'), (err && err.message) || '');
    }
  }

  function reply(m) {
    const sender = m.from && m.from[0] ? m.from[0].email : '';
    const subj = m.subject || '';
    const quote = body && body.text ? '\n\n> ' + body.text.split('\n').join('\n> ') : '';
    setComposing({ to: sender, subject: subj.startsWith('Re:') ? subj : 'Re: ' + subj, text: quote });
  }

  const filtered = q
    ? messages.filter(m => (addrLabel(m.from) + ' ' + (m.subject || '')).toLowerCase().includes(q.toLowerCase()))
    : messages;

  return (
    <div style={{ display: 'flex', height: 'calc(100vh - 150px)', minHeight: 420, border: '1px solid var(--hair)', borderRadius: 12, overflow: 'hidden', background: 'var(--surface)' }}>
      {/* Folders */}
      <div style={{ width: 220, borderRight: '1px solid var(--hair)', padding: 10, overflow: 'auto', flex: 'none' }}>
        <Button variant="primary" block onClick={() => setComposing(true)} style={{ marginBottom: 12 }}>{t('webmail.compose')}</Button>
        {folders.map(f => {
          const key = (f.name || '').toLowerCase();
          const icon = FOLDER_ICON[key] || 'folder';
          return (
            <FolderItem key={f.name} label={f.name} active={f.name === folder}
              icon={<Icon name={icon} size={14} style={{ color: 'var(--faint)' }} />}
              onClick={() => setFolder(f.name)} style={{ cursor: 'pointer' }} />
          );
        })}
        <div className="mf-u-faint mf-u-mono mf-truncate" style={{ fontSize: 11, padding: '14px 10px 4px' }}>{email}</div>
        <div style={{ padding: '0 10px' }}>
          <Button variant="link" size="sm" onClick={logout}>{t('webmail.signOut')}</Button>
        </div>
      </div>

      {/* Message list */}
      <div style={{ width: 360, borderRight: '1px solid var(--hair)', display: 'flex', flexDirection: 'column', flex: 'none', minHeight: 0 }}>
        <div style={{ padding: 10, borderBottom: '1px solid var(--hair)' }}>
          <SearchInput sm placeholder={t('webmail.search')} value={q} onChange={e => setQ(e.target.value)} />
        </div>
        <div style={{ overflow: 'auto', flex: 1 }}>
          {loadingList ? <Loading /> : error ? <ErrorState error={error} onRetry={() => loadMessages(folder)} />
            : filtered.length === 0 ? <Empty message={t('webmail.empty')} />
              : filtered.map(m => (
                <MailListItem key={m.uid}
                  from={addrLabel(m.from) || t('webmail.noSubject')}
                  subject={m.subject || t('webmail.noSubject')}
                  preview=""
                  time={shortTime(m.date)}
                  unread={!hasFlag(m.flags, '\\Seen')}
                  starred={hasFlag(m.flags, '\\Flagged')}
                  active={selected && selected.uid === m.uid}
                  onClick={() => openMessage(m)} />
              ))}
        </div>
      </div>

      {/* Reading pane */}
      <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        {!selected ? (
          <div style={{ margin: 'auto', color: 'var(--faint)', fontSize: 14 }}>{t('webmail.selectPrompt')}</div>
        ) : (
          <>
            <div style={{ padding: '18px 22px', borderBottom: '1px solid var(--hair)' }}>
              <div className="mf-row mf-row--between">
                <div className="mf-page-head__title" style={{ fontSize: 20 }}>{selected.subject || t('webmail.noSubject')}</div>
                <div className="mf-row" style={{ gap: 4 }}>
                  <Button variant="secondary" size="sm" onClick={() => reply(selected)}>{t('webmail.reply')}</Button>
                  <Button variant="ghost" size="sm" onClick={e => toggleStar(selected, e)} title={t('webmail.star')}><Icon name="star" size={15} style={{ color: hasFlag(selected.flags, '\\Flagged') ? 'var(--amber)' : 'var(--faint)' }} /></Button>
                  <Button variant="ghost" size="sm" onClick={() => archive(selected)} title={t('webmail.archive')}><Icon name="archive" size={15} /></Button>
                  <Button variant="ghost" size="sm" onClick={() => del(selected)} title={t('webmail.delete')}><Icon name="trash" size={15} /></Button>
                </div>
              </div>
              <div className="mf-row" style={{ gap: 10, marginTop: 10 }}>
                <Avatar size={34}>{initials(addrLabel(selected.from) || '?')}</Avatar>
                <div className="mf-min0">
                  <div className="mf-cell-name mf-truncate">{addrLabel(selected.from)}</div>
                  <div className="mf-cell-sub mf-truncate">{selected.from && selected.from[0] ? selected.from[0].email : ''}</div>
                </div>
                <span className="mf-u-faint mf-u-mono mf-spacer" style={{ fontSize: 12 }}>{shortTime(selected.date)}</span>
              </div>
            </div>
            <div style={{ overflow: 'auto', flex: 1, display: 'flex', flexDirection: 'column' }}>
              {body === null ? (
                <div style={{ padding: 22 }}><Loading message={t('webmail.loadingMessage')} /></div>
              ) : body.html ? (
                // Render HTML mail in a fully-sandboxed iframe: sandbox="" blocks
                // scripts, forms, popups and same-origin access, so untrusted mail
                // markup cannot run or reach the app.
                <iframe title="message" sandbox="" srcDoc={body.html}
                  style={{ border: 'none', width: '100%', flex: 1, minHeight: 320, background: '#fff' }} />
              ) : (
                <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word', font: '14px/1.6 var(--font-sans)', color: 'var(--ink)', margin: 0, padding: 22 }}>{body.text || ''}</pre>
              )}
              {body && Array.isArray(body.attachments) && body.attachments.length > 0 && (
                <div style={{ margin: '0 22px 22px', borderTop: '1px solid var(--hair)', paddingTop: 14 }}>
                  <div className="mf-u-muted" style={{ fontSize: 12, marginBottom: 8 }}>{t('webmail.attachments', { count: body.attachments.length })}</div>
                  <div className="mf-row" style={{ gap: 8, flexWrap: 'wrap' }}>
                    {body.attachments.map((a, i) => (
                      <Button key={i} variant="secondary" size="sm"
                        onClick={() => downloadAttachment(folder, selected.uid, i, a.filename).catch(err => toast(t('webmail.actionFailed'), err.message))}>
                        <Icon name="download" size={13} style={{ marginRight: 6 }} />{a.filename || ('attachment-' + i)}
                      </Button>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </>
        )}
      </div>

      {composing && (
        <ComposeDrawer
          initial={typeof composing === 'object' ? composing : {}}
          onClose={() => setComposing(false)}
          onSent={() => loadMessages(folder)}
        />
      )}
    </div>
  );
}

export function WebmailPage() {
  const { status } = useWebmailAuth();
  if (status !== 'authed') return <WebmailLogin />;
  return <WebmailClient />;
}
