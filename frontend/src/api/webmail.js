// Webmail talks to /api/webmail/* with its OWN bearer token (a mailbox session,
// distinct from the admin session), so it does not go through the admin api
// client. The token is kept separately in localStorage.

// A user can hold several mailbox sessions at once (multiple Mailfold mailboxes
// and/or synced external accounts) and switch between them in one webmail UI. The
// sessions live in WM_ACCOUNTS as [{email, token, external}]; WM_ACTIVE names the
// selected one. The legacy single-session keys are migrated on first read.
const WM_ACCOUNTS = 'mailfold.webmail.accounts';
const WM_ACTIVE = 'mailfold.webmail.active';
const WM_TOKEN = 'mailfold.webmail.token';
const WM_EMAIL = 'mailfold.webmail.email';

function writeAccounts(list) {
  try { localStorage.setItem(WM_ACCOUNTS, JSON.stringify(list)); } catch { /* storage may be unavailable */ }
}
function readAccounts() {
  let list = [];
  try { list = JSON.parse(localStorage.getItem(WM_ACCOUNTS) || '[]'); } catch { list = []; }
  if (!Array.isArray(list)) list = [];
  if (list.length === 0) {
    // Migrate a pre-multi-account single session into the accounts list.
    const tok = localStorage.getItem(WM_TOKEN);
    const em = localStorage.getItem(WM_EMAIL);
    if (tok && em) {
      list = [{ email: em, token: tok, external: false }];
      writeAccounts(list);
      localStorage.setItem(WM_ACTIVE, em);
    }
  }
  return list;
}

export function getAccounts() { return readAccounts(); }
export function getActiveEmail() {
  const list = readAccounts();
  const a = localStorage.getItem(WM_ACTIVE) || '';
  if (a && list.some(x => x.email === a)) return a;
  return list[0] ? list[0].email : '';
}
export function setActiveAccount(email) {
  try { localStorage.setItem(WM_ACTIVE, email || ''); } catch { /* ignore */ }
}
export function addAccount(email, token, external = false) {
  const list = readAccounts().filter(x => x.email !== email);
  list.push({ email, token, external: !!external });
  writeAccounts(list);
  setActiveAccount(email);
  return list;
}
export function removeAccount(email) {
  const list = readAccounts().filter(x => x.email !== email);
  writeAccounts(list);
  if (getActiveEmail() === email || !list.some(x => x.email === getActiveEmail())) {
    setActiveAccount(list[0] ? list[0].email : '');
  }
  return list;
}
// getWebmailToken returns the ACTIVE account's token — every /api/webmail/* call
// is made as the selected mailbox.
export function getWebmailToken() {
  const acc = readAccounts().find(x => x.email === getActiveEmail());
  return acc ? acc.token : '';
}
// Legacy shims kept so existing imports resolve.
export function setWebmailToken(token) { if (!token) removeAccount(getActiveEmail()); }
export function getWebmailEmail() { return getActiveEmail(); }
export function setWebmailEmail() { /* the active email is derived from the accounts store */ }

async function req(method, path, body) {
  const headers = { Accept: 'application/json' };
  const token = getWebmailToken();
  if (token) headers.Authorization = 'Bearer ' + token;
  let payload;
  if (body !== undefined && body !== null) {
    headers['Content-Type'] = 'application/json';
    payload = JSON.stringify(body);
  }
  const res = await fetch(path, { method, headers, body: payload });
  const text = await res.text();
  let data = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
    }
  }
  if (!res.ok) {
    const err = new Error((data && data.error) || res.statusText || 'request failed');
    err.status = res.status;
    err.body = data;
    throw err;
  }
  return data;
}

const q = params =>
  Object.entries(params)
    .filter(([, v]) => v !== undefined && v !== null && v !== '')
    .map(([k, v]) => `${k}=${encodeURIComponent(v)}`)
    .join('&');

// downloadAttachment fetches an attachment with the webmail token (a plain link
// cannot send the Authorization header) and triggers a browser download.
export async function downloadAttachment(folder, uid, index, filename) {
  const res = await fetch('/api/webmail/attachment?' + q({ folder, uid, index }), {
    headers: { Authorization: 'Bearer ' + getWebmailToken() },
  });
  if (!res.ok) throw new Error('download failed');
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename || 'attachment';
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

export const wm = {
  // login/logout do not persist the token themselves — the auth context does.
  login: (email, password) => req('POST', '/api/webmail/login', { email, password }),
  // deviceLogin exchanges a personal Mailfold API key for a webmail session —
  // signing into a new device without typing (or even knowing) the mailbox
  // password. Same {token, email} shape as login().
  deviceLogin: key => req('POST', '/api/auth/device-login', { key }),
  logout: () => req('POST', '/api/webmail/logout'),
  folders: () => req('GET', '/api/webmail/folders'),
  messages: (folder, limit = 50) => req('GET', '/api/webmail/messages?' + q({ folder, limit })),
  message: (folder, uid) => req('GET', '/api/webmail/message?' + q({ folder, uid })),
  search: (folder, query) => req('GET', '/api/webmail/search?' + q({ folder, q: query })),
  send: msg => req('POST', '/api/webmail/send', msg),
  flag: (folder, uid, flag, set) => req('POST', '/api/webmail/flag', { folder, uid, flag, set }),
  del: (folder, uid) => req('POST', '/api/webmail/delete', { folder, uid }),
  move: (folder, uid, target) => req('POST', '/api/webmail/move', { folder, uid, target }),
  createFolder: name => req('POST', '/api/webmail/folders', { name }),
  connectExternal: payload => req('POST', '/api/webmail/external', payload),
  calendar: {
    list: () => req('GET', '/api/webmail/calendar/events'),
    create: ev => req('POST', '/api/webmail/calendar/events', ev),
    update: (uid, ev) => req('PUT', '/api/webmail/calendar/events/' + encodeURIComponent(uid), ev),
    del: uid => req('DELETE', '/api/webmail/calendar/events/' + encodeURIComponent(uid)),
    setRsvp: (uid, rsvp) => req('PATCH', '/api/webmail/calendar/events/' + encodeURIComponent(uid) + '/rsvp', { rsvp }),
    // downloadAttachment streams a stored event attachment with the webmail
    // token (a plain link cannot send the Authorization header).
    downloadAttachment: async (uid, index, filename) => {
      const res = await fetch(
        '/api/webmail/calendar/events/' + encodeURIComponent(uid) + '/attachments/' + index,
        { headers: { Authorization: 'Bearer ' + getWebmailToken() } },
      );
      if (!res.ok) throw new Error('download failed');
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename || 'attachment';
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    },
  },
  signature: {
    get: () => req('GET', '/api/webmail/signature'),
    set: signature => req('PUT', '/api/webmail/signature', { signature }),
  },
  rules: {
    list: () => req('GET', '/api/webmail/rules'),
    create: rule => req('POST', '/api/webmail/rules', rule),
    update: (id, rule) => req('PUT', '/api/webmail/rules', { id: String(id), ...rule }),
    del: id => req('DELETE', '/api/webmail/rules', { id: String(id) }),
  },
  totp: {
    status: () => req('GET', '/api/webmail/2fa/status'),
    enroll: currentPassword => req('POST', '/api/webmail/2fa/enroll', { current_password: currentPassword }),
    confirm: code => req('POST', '/api/webmail/2fa/confirm', { code }),
    disable: currentPassword => req('POST', '/api/webmail/2fa/disable', { current_password: currentPassword }),
    regenerateRecoveryCodes: () => req('POST', '/api/webmail/2fa/recovery-codes'),
    // verify does not use req(): it runs before any session token exists.
    verify: (pendingToken, code) => req('POST', '/api/webmail/2fa/verify', { pending_token: pendingToken, code }),
  },
};

// subscribeMail opens a Server-Sent Events stream that fires onMail(data) when
// new INBOX mail arrives ({ count, messages }). The token goes in the query
// because EventSource cannot set an Authorization header. Returns an unsubscribe
// function; a no-op when there is no session or SSE is unavailable.
export function subscribeMail(onMail) {
  const token = getWebmailToken();
  if (!token || typeof EventSource === 'undefined') return () => {};
  const es = new EventSource('/api/webmail/events?token=' + encodeURIComponent(token));
  es.addEventListener('mail', e => {
    try { onMail(JSON.parse(e.data)); } catch { /* ignore a malformed event */ }
  });
  return () => es.close();
}
