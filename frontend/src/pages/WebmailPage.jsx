import React, { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { FolderItem } from '../ds/components/molecules/FolderItem.jsx';
import { LabelItem } from '../ds/components/molecules/LabelItem.jsx';
import { IconButton } from '../ds/components/atoms/IconButton.jsx';
import { Checkbox } from '../ds/components/atoms/Checkbox.jsx';
import { MailListItem } from '../ds/components/molecules/MailListItem.jsx';
import { SearchInput } from '../ds/components/molecules/SearchInput.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { PasswordField } from '../components/PasswordField.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Segmented } from '../ds/components/atoms/Segmented.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { Avatar } from '../ds/components/atoms/Avatar.jsx';
import { initials } from '../ds/data/sample.js';
import { useWebmailAuth } from '../auth/WebmailAuthContext.jsx';
import { useAuth } from '../auth/AuthContext.jsx';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';
import { wm, downloadAttachment, subscribeMail } from '../api/webmail.js';
import { ConfirmModal } from '../ds/components/organisms/ConfirmModal.jsx';
import { ComposeModal } from './ComposeModal.jsx';
import { UndoSendBar } from './UndoSendBar.jsx';
import { AddAccountModal } from './AddAccountModal.jsx';
import { WebmailSettingsDrawer } from './WebmailSettingsDrawer.jsx';
import { CalendarView } from './CalendarView.jsx';
import { Loading, ErrorState, Empty } from '../components/States.jsx';
import { decodeIdnAddress } from '../lib/idn.js';
import { collapseQuotedHtml, collapseQuotedText } from '../lib/quotes.js';

const SYS_ICON = { inbox: 'inbox', sent: 'send', drafts: 'drafts', archive: 'archive', junk: 'shield', spam: 'shield', trash: 'trash' };
const SYS_ORDER = ['inbox', 'sent', 'drafts', 'archive', 'junk', 'spam', 'trash'];
const SYS_ATTRS = ['\\sent', '\\drafts', '\\junk', '\\trash', '\\archive', '\\all'];
const LABEL_PALETTE = ['#B07C33', '#4B7B58', '#3C6187', '#9B5A4A', '#8A6D3B', '#6E6860'];

function sysRank(name) {
  const i = SYS_ORDER.indexOf((name || '').toLowerCase());
  return i === -1 ? 99 : i;
}
// classifyFolders splits IMAP folders into ordered system folders and the rest.
function classifyFolders(folders) {
  const sys = [];
  const custom = [];
  (folders || []).forEach(f => {
    const key = (f.name || '').toLowerCase();
    const attrs = (f.attributes || []).map(a => a.toLowerCase());
    const isSys = SYS_ORDER.includes(key) || attrs.some(a => SYS_ATTRS.includes(a));
    (isSys ? sys : custom).push(f);
  });
  sys.sort((a, b) => sysRank(a.name) - sysRank(b.name));
  return { sys, custom };
}
// folderLeaf returns the last segment of a hierarchical IMAP folder name.
function folderLeaf(name) {
  const parts = String(name || '').split(/[/.]/);
  return parts[parts.length - 1] || name;
}
const labelStoreKey = email => 'mailfold.webmail.labels.' + (email || '');
function loadLabels(email) {
  try { return JSON.parse(localStorage.getItem(labelStoreKey(email)) || '[]'); } catch { return []; }
}
function saveLabels(email, labels) {
  try { localStorage.setItem(labelStoreKey(email), JSON.stringify(labels)); } catch { /* storage may be unavailable */ }
}

const hasFlag = (flags, f) => Array.isArray(flags) && flags.includes(f);
// addrLabel is display-only (search filtering and JSX text) — never feed its
// result into a compose "to" field or any other request; reply()/forward()
// deliberately read m.from[0].email directly instead, for exactly that reason.
const addrLabel = list => {
  const a = Array.isArray(list) && list[0];
  return a ? (a.name || decodeIdnAddress(a.email || '')) : '';
};
const shortTime = iso => {
  const d = new Date(iso);
  if (isNaN(d)) return '';
  const now = new Date();
  return d.toDateString() === now.toDateString()
    ? d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    : d.toLocaleDateString([], { month: 'short', day: 'numeric' });
};
const fullDateTime = iso => {
  const d = new Date(iso);
  return isNaN(d) ? '' : d.toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' });
};
// addressListText is display-only, for quoted reply/forward headers — never
// fed back into a request (the actual To/Cc fields are built from raw
// addresses elsewhere).
const addressListText = list => {
  if (!Array.isArray(list) || list.length === 0) return '';
  return list.map(a => {
    const email = decodeIdnAddress(a.email || '');
    return a.name ? `${a.name} <${email}>` : email;
  }).join(', ');
};

// Inline mailbox login shown when there is no webmail session (e.g. an admin
// opening the Webmail page).
function WebmailLogin() {
  const { login, verifyLogin2FA } = useWebmailAuth();
  const t = useT();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [pending, setPending] = useState(null); // { pendingToken } once this mailbox's own 2FA is required
  const [code, setCode] = useState('');
  const [codeError, setCodeError] = useState('');

  async function submit(e) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError('');
    try {
      const res = await login(email.trim(), password);
      if (res && res.needs_2fa) setPending({ pendingToken: res.pending_token });
    } catch (err) {
      setError(err && err.status === 401 ? t('webmail.invalid') : (err.message || t('webmail.invalid')));
    } finally {
      setBusy(false);
    }
  }

  async function submitCode(e) {
    e.preventDefault();
    if (busy || !code.trim()) return;
    setBusy(true);
    setCodeError('');
    try {
      await verifyLogin2FA(pending.pendingToken, code.trim(), email.trim());
    } catch {
      setCodeError(t('login.twoFactor.invalidCode'));
      setBusy(false);
    }
  }

  if (pending) {
    return (
      <div style={{ maxWidth: 380, margin: '8vh auto 0' }}>
        <form onSubmit={submitCode}>
          <div className="mf-login__title" style={{ fontSize: 24 }}>{t('login.twoFactor.title')}</div>
          <div className="mf-login__sub">{t('login.twoFactor.sub')}</div>
          <FormField label={t('login.twoFactor.codeLabel')} style={{ marginTop: 22 }}>
            <Input size="lg" mono autoFocus placeholder={t('login.twoFactor.codePlaceholder')} autoComplete="one-time-code" value={code} onChange={e => setCode(e.target.value)} />
          </FormField>
          {codeError && <div className="mf-form-error" style={{ marginTop: 14 }} role="alert">{codeError}</div>}
          <Button variant="primary" block size="lg" type="submit" disabled={busy} style={{ marginTop: 22 }}>{busy ? t('login.twoFactor.verifying') : t('login.twoFactor.verify')}</Button>
        </form>
      </div>
    );
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
        {error && <div className="mf-form-error" style={{ marginTop: 14 }} role="alert">{error}</div>}
        <Button variant="primary" block size="lg" type="submit" disabled={busy} style={{ marginTop: 22 }}>{busy ? t('webmail.signingIn') : t('webmail.signIn')}</Button>
      </form>
    </div>
  );
}

function WebmailClient() {
  const t = useT();
  const { email, accounts, switchAccount, expire, logout } = useWebmailAuth();
  const { toast } = useToast();
  const [addingAccount, setAddingAccount] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [confirmLogout, setConfirmLogout] = useState(false);

  const [folders, setFolders] = useState([]);
  const [folder, setFolder] = useState('INBOX');
  const [messages, setMessages] = useState([]);
  const [selected, setSelected] = useState(null); // MessageHeader
  const [body, setBody] = useState(null);
  const [showAllQuoted, setShowAllQuoted] = useState(false);
  const [q, setQ] = useState('');
  const [loadingList, setLoadingList] = useState(true);
  const [error, setError] = useState(null);
  const [composing, setComposing] = useState(false);
  const [view, setView] = useState('mail'); // 'mail' | 'calendar'
  const [filterMode, setFilterMode] = useState(null); // null | 'starred' | 'label:<name>'
  const [labels, setLabels] = useState(() => loadLabels(email));
  // sharedMembers is non-null only inside a shared/team mailbox (see
  // wm.shared.members()); it gates the assignment/notes UI in the reading
  // pane below. WebmailClient remounts (key={email}, see WebmailPage at the
  // bottom of this file) on every account switch, so a plain mount-time
  // fetch is enough — no need to re-check on every render.
  const [sharedMembers, setSharedMembers] = useState(null);
  const [notes, setNotes] = useState([]);
  const [noteDraft, setNoteDraft] = useState('');
  // pendingUndo drives the UndoSendBar — set from ComposeModal's onSent()
  // for a normal (implicit undo-window) send, cleared when the bar dismisses.
  const [pendingUndo, setPendingUndo] = useState(null);
  const [scheduled, setScheduled] = useState([]);
  const [cancelTarget, setCancelTarget] = useState(null); // scheduled item pending cancel confirmation

  useEffect(() => {
    wm.shared.members().then(setSharedMembers).catch(() => setSharedMembers(null));
  }, []);

  const loadScheduled = useCallback(() => {
    wm.listScheduled().then(list => setScheduled(Array.isArray(list) ? list : [])).catch(() => {});
  }, []);
  useEffect(() => { loadScheduled(); }, [loadScheduled]);

  const { sys: sysFolders, custom: customFolders } = useMemo(() => classifyFolders(folders), [folders]);

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
    const who = first && first.from && first.from[0] ? (first.from[0].name || decodeIdnAddress(first.from[0].email || '')) : '';
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
    setShowAllQuoted(false);
    setNotes([]);
    setNoteDraft('');
    try {
      const full = await wm.message(folder, m.uid);
      setBody(full);
      if (!hasFlag(m.flags, '\\Seen')) {
        // The write API takes friendly flag names ('seen'); the flags array
        // returned by the read API uses raw IMAP flags ('\\Seen').
        await wm.flag(folder, m.uid, 'seen', true);
        setMessages(list => list.map(x => (x.uid === m.uid ? { ...x, flags: [...(x.flags || []), '\\Seen'] } : x)));
      }
      if (sharedMembers !== null) {
        wm.shared.notes(folder, m.uid).then(setNotes).catch(() => setNotes([]));
      }
    } catch (err) {
      onErr(err);
    }
  }

  // assignMessage sets (or, with an empty value, clears) who on the team is
  // handling the selected message — only reachable from inside a shared
  // mailbox (see sharedMembers above).
  async function assignMessage(assignedTo) {
    if (!selected) return;
    try {
      await wm.shared.setAssignment(folder, selected.uid, assignedTo);
      setMessages(list => list.map(x => (x.uid === selected.uid ? { ...x, assigned_to: assignedTo } : x)));
      setSelected(s => (s ? { ...s, assigned_to: assignedTo } : s));
    } catch (err) {
      toast(t('webmail.actionFailed'), (err && err.message) || '');
    }
  }

  async function addNote() {
    const body = noteDraft.trim();
    if (!selected || !body) return;
    try {
      const note = await wm.shared.addNote(folder, selected.uid, body);
      setNotes(list => [...list, note]);
      setNoteDraft('');
      setMessages(list => list.map(x => (x.uid === selected.uid ? { ...x, notes_count: (x.notes_count || 0) + 1 } : x)));
    } catch (err) {
      toast(t('webmail.actionFailed'), (err && err.message) || '');
    }
  }

  async function deleteNote(id) {
    try {
      await wm.shared.deleteNote(id);
      setNotes(list => list.filter(n => n.id !== id));
      setMessages(list => list.map(x => (x.uid === selected.uid ? { ...x, notes_count: Math.max(0, (x.notes_count || 0) - 1) } : x)));
    } catch (err) {
      toast(t('webmail.actionFailed'), (err && err.message) || '');
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

  async function cancelScheduledItem(item) {
    try {
      await wm.cancelScheduled(item.id);
      toast(t('webmail.scheduled.canceled'));
      loadScheduled();
    } catch (err) {
      toast(t('webmail.actionFailed'), (err && err.message) || '');
    } finally {
      setCancelTarget(null);
    }
  }

  function reply(m) {
    const sender = m.from && m.from[0] ? m.from[0].email : '';
    const subj = m.subject || '';
    let quote = '';
    if (body && body.text) {
      const header = t('webmail.quote.on', { date: fullDateTime(m.date), sender: addressListText(m.from) });
      const quoted = body.text.split('\n').map(l => '> ' + l).join('\n');
      quote = '\n\n' + header + '\n' + quoted;
    }
    setComposing({ to: sender, subject: subj.startsWith('Re:') ? subj : 'Re: ' + subj, text: quote });
  }

  function forward(m) {
    const subj = m.subject || '';
    let quote = '';
    if (body && body.text) {
      const header = [
        t('webmail.quote.forwardHeader'),
        t('webmail.quote.from', { value: addressListText(m.from) }),
        t('webmail.quote.date', { value: fullDateTime(m.date) }),
        t('webmail.quote.subject', { value: subj }),
        t('webmail.quote.to', { value: addressListText(m.to) }),
      ].join('\n');
      quote = '\n\n' + header + '\n\n' + body.text;
    }
    setComposing({ subject: subj.startsWith('Fwd:') ? subj : 'Fwd: ' + subj, text: quote });
  }

  function selectFolder(name) { setFilterMode(null); setFolder(name); }
  function selectScheduled() { setFilterMode('scheduled'); loadScheduled(); }

  async function createFolder() {
    const name = (window.prompt(t('webmail.folderNamePrompt')) || '').trim();
    if (!name) return;
    try {
      await wm.createFolder(name);
      toast(t('webmail.folderCreated', { name }));
      wm.folders().then(setFolders).catch(onErr);
    } catch (err) {
      toast(t('webmail.createFolderFailed'), (err && err.body && err.body.error) || (err && err.message) || '');
    }
  }

  function createLabel() {
    const name = (window.prompt(t('webmail.labelNamePrompt')) || '').trim();
    if (!name || labels.some(l => l.name === name)) return;
    const next = [...labels, { name, color: LABEL_PALETTE[labels.length % LABEL_PALETTE.length] }];
    setLabels(next);
    saveLabels(email, next);
    toast(t('webmail.labelCreated', { name }));
  }

  // The message list is filtered by the active favourite/label view and the
  // search box. Labels match on IMAP keywords, which the server returns in each
  // message's flags array, so filtering is a client-side flag check.
  let base = messages;
  if (filterMode === 'starred') base = base.filter(m => hasFlag(m.flags, '\\Flagged'));
  else if (filterMode && filterMode.startsWith('label:')) {
    const lbl = filterMode.slice(6);
    base = base.filter(m => hasFlag(m.flags, lbl));
  }
  const filtered = q
    ? base.filter(m => (addrLabel(m.from) + ' ' + (m.subject || '')).toLowerCase().includes(q.toLowerCase()))
    : base;
  const unreadCount = messages.filter(m => !hasFlag(m.flags, '\\Seen')).length;
  const listTitle = filterMode === 'starred'
    ? t('webmail.starred')
    : filterMode === 'scheduled' ? t('webmail.scheduled.title')
    : filterMode && filterMode.startsWith('label:') ? filterMode.slice(6) : folderLeaf(folder);

  // Collapse deep quote history once here rather than in the render below —
  // parsing HTML is not free, and the result only changes when the message
  // (or its body) changes, not on every re-render.
  const collapsedHtml = useMemo(
    () => (body && body.html
      ? collapseQuotedHtml(body.html, n => t('webmail.quote.showEarlier', { count: n }))
      : null),
    [body, t],
  );
  const collapsedText = useMemo(
    () => (body && !body.html ? collapseQuotedText(body.text || '') : null),
    [body],
  );

  return (
    <>
      {view === 'calendar' ? <CalendarView onAppView={setView} /> : (
      <>
      <div style={{ marginBottom: 12 }}>
        <Segmented options={[t('webmail.view.mail'), t('webmail.view.calendar')]} value={t('webmail.view.mail')}
          onSelect={v => setView(v === t('webmail.view.calendar') ? 'calendar' : 'mail')} className="mf-cal-appseg" style={{ display: 'inline-flex' }} />
      </div>
    <div className="mf-webmail" style={{ height: 'calc(100vh - 190px)', minHeight: 460, border: '1px solid var(--hair)', borderRadius: 12, overflow: 'hidden', background: 'var(--surface)' }}>
      {/* Folder rail */}
      <div className="mf-webmail__folders">
        <Button variant="primary" block onClick={() => setComposing(true)} style={{ margin: '2px 0 12px' }}>{t('webmail.compose')}</Button>

        <div className="mf-side-label" style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <Icon name="chevron-down" size={11} style={{ color: 'var(--faint)' }} />{t('webmail.account.accounts')}
        </div>
        {accounts.map(acc => (
          <div key={acc.email} onClick={() => switchAccount(acc.email)}
            className="mf-row" style={{ gap: 8, padding: '6px 10px', borderRadius: 8, cursor: 'pointer', background: acc.email === email ? 'var(--accent-soft)' : 'transparent' }}>
            <Avatar size={22}>{initials(acc.email)}</Avatar>
            <span className="mf-u-mono mf-truncate" style={{ flex: 1, fontSize: 11.5, fontWeight: 600, color: acc.email === email ? 'var(--accent-ink)' : 'var(--muted)' }}>{decodeIdnAddress(acc.email)}</span>
          </div>
        ))}
        <div onClick={() => setAddingAccount(true)} className="mf-row" style={{ gap: 9, padding: '6px 10px', color: 'var(--accent-ink)', font: '600 12.5px var(--font-sans)', cursor: 'pointer' }}>
          <span style={{ width: 22, height: 22, borderRadius: 6, border: '1px dashed var(--hair)', display: 'flex', alignItems: 'center', justifyContent: 'center', flex: 'none', fontSize: 15, lineHeight: 1 }}>+</span>{t('webmail.account.add')}
        </div>

        <div className="mf-side-label">{t('webmail.favourites')}</div>
        <FolderItem icon={<Icon name="star" size={14} style={{ color: 'var(--amber)' }} />} label={t('webmail.starred')}
          active={filterMode === 'starred'} onClick={() => setFilterMode('starred')} style={{ cursor: 'pointer' }} />
        <FolderItem icon={<Icon name="flag" size={14} style={{ color: 'var(--red)' }} />} label={t('webmail.flagged')}
          onClick={() => setFilterMode('starred')} style={{ cursor: 'pointer' }} />
        <FolderItem icon={<Icon name="clock" size={14} style={{ color: 'var(--faint)' }} />} label={t('webmail.snoozed')}
          onClick={() => toast(t('webmail.snoozeUnavailable'))} style={{ cursor: 'pointer' }} />
        <FolderItem icon={<Icon name="clock" size={14} style={{ color: filterMode === 'scheduled' ? 'var(--accent-ink)' : 'var(--faint)' }} />} label={t('webmail.scheduled.title')}
          active={filterMode === 'scheduled'} count={scheduled.length > 0 ? scheduled.length : undefined}
          onClick={selectScheduled} style={{ cursor: 'pointer' }} />

        {sysFolders.map(f => {
          const isActive = f.name === folder && !filterMode;
          return (
            <FolderItem key={f.name} label={folderLeaf(f.name)} active={isActive}
              count={isActive && unreadCount ? unreadCount : undefined}
              icon={<Icon name={SYS_ICON[(f.name || '').toLowerCase()] || 'folder'} size={15} style={{ color: isActive ? 'var(--accent-ink)' : 'var(--faint)' }} />}
              onClick={() => selectFolder(f.name)} style={{ cursor: 'pointer' }} />
          );
        })}

        {customFolders.length > 0 && <div className="mf-side-label">{t('webmail.folders')}</div>}
        {customFolders.map(f => (
          <FolderItem key={f.name} label={folderLeaf(f.name)} nested={/[/.]/.test(f.name)} active={f.name === folder && !filterMode}
            icon={<Icon name="folder" size={15} style={{ color: f.name === folder && !filterMode ? 'var(--accent-ink)' : 'var(--faint)' }} />}
            onClick={() => selectFolder(f.name)} style={{ cursor: 'pointer' }} />
        ))}
        <div className="mf-row" onClick={createFolder} style={{ gap: 9, padding: '7px 10px', font: '600 12.5px var(--font-sans)', color: 'var(--accent-ink)', cursor: 'pointer' }}>
          <span style={{ width: 15, textAlign: 'center' }}>+</span>{t('webmail.newFolder')}
        </div>

        <div className="mf-side-label">{t('webmail.labels')}</div>
        {labels.map(l => (
          <LabelItem key={l.name} color={l.color} label={l.name}
            className={filterMode === 'label:' + l.name ? 'mf-folder--active' : ''}
            onClick={() => setFilterMode('label:' + l.name)} style={{ cursor: 'pointer' }} />
        ))}
        <div className="mf-row" onClick={createLabel} style={{ gap: 9, padding: '7px 10px', font: '600 12.5px var(--font-sans)', color: 'var(--accent-ink)', cursor: 'pointer' }}>
          <span style={{ width: 15, textAlign: 'center' }}>+</span>{t('webmail.newLabel')}
        </div>

        <div style={{ padding: '12px 10px 0', display: 'flex', gap: 6 }}>
          <Button variant="link" size="sm" onClick={() => setSettingsOpen(true)}>{t('webmail.settings.open')}</Button>
          <Button variant="link" size="sm" onClick={() => setConfirmLogout(true)}>{t('webmail.signOut')}</Button>
        </div>
      </div>

      {/* Message list */}
      <div className="mf-webmail__list">
        <div className="mf-webmail__list-head">
          <Checkbox />
          <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>{listTitle}</span>
          {filterMode === 'scheduled'
            ? <span className="mf-u-faint" style={{ fontSize: 12 }}>{scheduled.length}</span>
            : <span className="mf-u-faint" style={{ fontSize: 12 }}>{unreadCount} {t('webmail.unread')}</span>}
          {filterMode !== 'scheduled' && (
            <SearchInput sm className="mf-spacer" placeholder={t('webmail.search')} value={q} onChange={e => setQ(e.target.value)} style={{ width: 150 }} />
          )}
        </div>
        <div style={{ overflow: 'auto', flex: 1 }}>
          {filterMode === 'scheduled' ? (
            scheduled.length === 0 ? <Empty message={t('webmail.scheduled.empty')} /> : (
              scheduled.map(item => (
                <div key={item.id} className="mf-row" style={{ gap: 10, padding: '10px 16px', borderBottom: '1px solid var(--hair-soft)', alignItems: 'flex-start' }}>
                  <div className="mf-min0" style={{ flex: 1 }}>
                    <div className="mf-truncate" style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>{(item.to || []).join(', ') || t('webmail.noSubject')}</div>
                    <div className="mf-truncate" style={{ fontSize: 12.5, color: 'var(--muted)' }}>{item.subject || t('webmail.noSubject')}</div>
                    <div className="mf-u-faint" style={{ fontSize: 11.5, marginTop: 2 }}>{t('webmail.scheduled.scheduledFor', { time: fullDateTime(item.scheduledAt) })}</div>
                  </div>
                  <Button variant="link" size="sm" onClick={() => setCancelTarget(item)}>{t('common.cancel')}</Button>
                </div>
              ))
            )
          ) : loadingList ? <Loading /> : error ? <ErrorState error={error} onRetry={() => loadMessages(folder)} />
            : filtered.length === 0 ? <Empty message={t('webmail.empty')} />
              : filtered.map(m => (
                <MailListItem key={m.uid}
                  from={addrLabel(m.from) || t('webmail.noSubject')}
                  subject={m.subject || t('webmail.noSubject')}
                  preview={m.preview || ''}
                  time={shortTime(m.date)}
                  unread={!hasFlag(m.flags, '\\Seen')}
                  starred={hasFlag(m.flags, '\\Flagged')}
                  assignedTo={m.assigned_to || ''}
                  notesCount={m.notes_count || 0}
                  active={selected && selected.uid === m.uid}
                  onClick={() => openMessage(m)} />
              ))}
        </div>
      </div>

      {/* Reading pane */}
      <div className="mf-webmail__reader">
        {!selected ? (
          <div style={{ margin: 'auto', color: 'var(--faint)', fontSize: 14 }}>{t('webmail.selectPrompt')}</div>
        ) : (
          <>
            <div className="mf-webmail__toolbar">
              <Button variant="primary" size="sm" onClick={() => reply(selected)}><Icon name="reply" size={14} style={{ marginRight: 6 }} />{t('webmail.reply')}</Button>
              <Button variant="secondary" size="sm" onClick={() => forward(selected)}><Icon name="forward" size={14} style={{ marginRight: 6 }} />{t('webmail.forward')}</Button>
              <div className="mf-spacer mf-row" style={{ gap: 2 }}>
                <IconButton onClick={() => archive(selected)} title={t('webmail.archive')}><Icon name="archive" size={16} /></IconButton>
                <IconButton onClick={() => del(selected)} title={t('webmail.delete')}><Icon name="trash" size={16} /></IconButton>
                <IconButton onClick={e => toggleStar(selected, e)} title={t('webmail.star')}><Icon name="star" size={16} style={{ color: hasFlag(selected.flags, '\\Flagged') ? 'var(--amber)' : 'var(--faint)' }} /></IconButton>
              </div>
            </div>
            <div className="mf-webmail__body" style={{ overflow: 'auto', flex: 1, display: 'flex', flexDirection: 'column' }}>
              <div style={{ fontFamily: 'var(--font-serif)', fontSize: 22, fontWeight: 600, color: 'var(--ink-strong)', lineHeight: 1.25 }}>{selected.subject || t('webmail.noSubject')}</div>
              <div className="mf-row" style={{ gap: 12, margin: '16px 0 20px' }}>
                <Avatar size={38}>{initials(addrLabel(selected.from) || '?')}</Avatar>
                <div className="mf-min0" style={{ flex: 1 }}>
                  <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--ink)' }}>{addrLabel(selected.from)}</div>
                  <div className="mf-u-faint mf-u-mono mf-truncate" style={{ fontSize: 12.5 }}>{selected.from && selected.from[0] ? selected.from[0].email : ''}</div>
                </div>
                <span className="mf-u-faint" style={{ fontSize: 12.5 }}>{shortTime(selected.date)}</span>
              </div>
              {sharedMembers !== null && (
                <div style={{ margin: '0 0 20px', padding: 14, borderRadius: 10, border: '1px solid var(--hair)', background: 'var(--surface-soft)' }}>
                  <div className="mf-row" style={{ gap: 10, alignItems: 'center' }}>
                    <span className="mf-u-faint" style={{ fontSize: 12, flex: 'none' }}>{t('webmail.shared.assignedTo')}</span>
                    <select className="mf-input" style={{ maxWidth: 220 }} value={selected.assigned_to || ''} onChange={e => assignMessage(e.target.value)}>
                      <option value="">{t('webmail.shared.unassigned')}</option>
                      {sharedMembers.map(m => <option key={m} value={m}>{m}</option>)}
                    </select>
                  </div>
                  <div className="mf-u-faint" style={{ fontSize: 12, margin: '12px 0 6px' }}>{t('webmail.shared.notes')}</div>
                  {notes.length === 0 && <div className="mf-u-faint" style={{ fontSize: 12.5 }}>{t('webmail.shared.notesEmpty')}</div>}
                  {notes.map(n => (
                    <div key={n.id} className="mf-row" style={{ gap: 8, padding: '6px 0', borderTop: '1px solid var(--hair-soft)', alignItems: 'flex-start' }}>
                      <div className="mf-min0" style={{ flex: 1 }}>
                        <div style={{ fontSize: 12.5, color: 'var(--ink)', whiteSpace: 'pre-wrap' }}>{n.body}</div>
                        <div className="mf-u-faint" style={{ fontSize: 11 }}>{n.author} · {shortTime(n.created_at)}</div>
                      </div>
                      <IconButton onClick={() => deleteNote(n.id)} title={t('common.delete')}><Icon name="trash" size={13} /></IconButton>
                    </div>
                  ))}
                  <div className="mf-row" style={{ gap: 8, marginTop: 10 }}>
                    <Input className="mf-spacer" placeholder={t('webmail.shared.notePlaceholder')} value={noteDraft}
                      onChange={e => setNoteDraft(e.target.value)}
                      onKeyDown={e => { if (e.key === 'Enter') addNote(); }} />
                    <Button variant="secondary" size="sm" onClick={addNote} disabled={!noteDraft.trim()}>{t('webmail.shared.addNote')}</Button>
                  </div>
                </div>
              )}
              {body === null ? (
                <div style={{ padding: 22 }}><Loading message={t('webmail.loadingMessage')} /></div>
              ) : body.html ? (
                // Render HTML mail in a fully-sandboxed iframe: sandbox="" blocks
                // scripts, forms, popups and same-origin access, so untrusted mail
                // markup cannot run or reach the app. Deep quote history is
                // pre-collapsed into a native <details> (see collapseQuotedHtml),
                // which toggles without JavaScript, so it still works here.
                <iframe title="message" sandbox="" srcDoc={collapsedHtml}
                  style={{ border: 'none', width: '100%', flex: 1, minHeight: 320, background: '#fff' }} />
              ) : (
                <>
                  <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word', font: '14px/1.6 var(--font-sans)', color: 'var(--ink)', margin: 0, padding: 22 }}>
                    {collapsedText.visible}
                  </pre>
                  {collapsedText.hiddenCount > 0 && (
                    <button
                      onClick={() => setShowAllQuoted(s => !s)}
                      style={{ margin: '0 22px 22px', alignSelf: 'flex-start', background: 'none', border: 'none', color: 'var(--accent-ink)', font: '600 12.5px var(--font-sans)', cursor: 'pointer', padding: 0 }}
                    >
                      {showAllQuoted ? t('webmail.quote.hideEarlier') : t('webmail.quote.showEarlier', { count: collapsedText.hiddenCount })}
                    </button>
                  )}
                  {showAllQuoted && (
                    <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word', font: '14px/1.6 var(--font-sans)', color: 'var(--ink)', margin: '0 0 22px', padding: '0 22px' }}>
                      {collapsedText.hidden}
                    </pre>
                  )}
                </>
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
              <div className="mf-reply-box" onClick={() => reply(selected)}>{t('webmail.replyTo', { name: addrLabel(selected.from) })}…</div>
            </div>
          </>
        )}
      </div>

      {composing && (
        <ComposeModal
          initial={typeof composing === 'object' ? composing : {}}
          onClose={() => setComposing(false)}
          onSent={scheduleInfo => {
            loadMessages(folder);
            loadScheduled();
            if (scheduleInfo) setPendingUndo(scheduleInfo);
          }}
        />
      )}
      {addingAccount && <AddAccountModal onClose={() => setAddingAccount(false)} />}
      {settingsOpen && <WebmailSettingsDrawer onClose={() => setSettingsOpen(false)} />}
      {confirmLogout && (
        <ConfirmModal
          title={t('webmail.signOutConfirm.title')}
          msg={t('webmail.signOutConfirm.msg')}
          cta={t('webmail.signOut')}
          onCancel={() => setConfirmLogout(false)}
          onConfirm={logout}
        />
      )}
      {cancelTarget && (
        <ConfirmModal
          title={t('webmail.scheduled.cancelTitle')}
          msg={t('webmail.scheduled.cancelMsg', { subject: cancelTarget.subject || t('webmail.noSubject') })}
          cta={t('common.cancel')}
          danger
          onCancel={() => setCancelTarget(null)}
          onConfirm={() => cancelScheduledItem(cancelTarget)}
        />
      )}
      {pendingUndo && (
        <UndoSendBar info={pendingUndo} onDismiss={() => { setPendingUndo(null); loadMessages(folder); loadScheduled(); }} />
      )}
    </div>
      </>
      )}
    </>
  );
}

// LinkAccountPrompt asks an admin, right after opening a mailbox's webmail
// for the first time, whether to keep it as one of their linked accounts
// (persists in the account switcher) or just view it this once (forgotten
// again once they leave webmail — see cleanupTemporary).
function LinkAccountPrompt({ email, onLink, onSkip }) {
  const t = useT();
  return (
    <div className="mf-overlay mf-overlay--center">
      <div className="mf-dialog" onClick={e => e.stopPropagation()}>
        <div className="mf-dialog__body">
          <div className="mf-dialog__title">{t('webmail.linkPrompt.title')}</div>
          <div className="mf-dialog__msg">{t('webmail.linkPrompt.sub', { email })}</div>
        </div>
        <div className="mf-dialog__foot">
          <Button variant="secondary" onClick={onSkip}>{t('webmail.linkPrompt.skip')}</Button>
          <Button variant="primary" onClick={onLink}>{t('webmail.linkPrompt.link')}</Button>
        </div>
      </div>
    </div>
  );
}

export function WebmailPage() {
  const { status, email, justAdded, clearJustAdded, markTemporary } = useWebmailAuth();
  const { status: adminStatus } = useAuth();
  if (status !== 'authed') return <WebmailLogin />;
  const showLinkPrompt = adminStatus === 'authed' && justAdded === email;
  return (
    <>
      {/* Key by the active account so switching mailboxes remounts the client
          with a clean folder/message state for the newly-selected session. */}
      <WebmailClient key={email} />
      {showLinkPrompt && (
        <LinkAccountPrompt email={email} onLink={clearJustAdded} onSkip={() => markTemporary(email)} />
      )}
    </>
  );
}
