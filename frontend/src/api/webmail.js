// Webmail talks to /api/webmail/* with its OWN bearer token (a mailbox session,
// distinct from the admin session), so it does not go through the admin api
// client. The token is kept separately in localStorage.

const WM_TOKEN = 'mailfold.webmail.token';
const WM_EMAIL = 'mailfold.webmail.email';

export function getWebmailToken() {
  return localStorage.getItem(WM_TOKEN) || '';
}
export function setWebmailToken(token) {
  if (token) localStorage.setItem(WM_TOKEN, token);
  else localStorage.removeItem(WM_TOKEN);
}
export function getWebmailEmail() {
  return localStorage.getItem(WM_EMAIL) || '';
}
export function setWebmailEmail(email) {
  if (email) localStorage.setItem(WM_EMAIL, email);
  else localStorage.removeItem(WM_EMAIL);
}

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
  logout: () => req('POST', '/api/webmail/logout'),
  folders: () => req('GET', '/api/webmail/folders'),
  messages: (folder, limit = 50) => req('GET', '/api/webmail/messages?' + q({ folder, limit })),
  message: (folder, uid) => req('GET', '/api/webmail/message?' + q({ folder, uid })),
  search: (folder, query) => req('GET', '/api/webmail/search?' + q({ folder, q: query })),
  send: msg => req('POST', '/api/webmail/send', msg),
  flag: (folder, uid, flag, set) => req('POST', '/api/webmail/flag', { folder, uid, flag, set }),
  del: (folder, uid) => req('POST', '/api/webmail/delete', { folder, uid }),
  move: (folder, uid, target) => req('POST', '/api/webmail/move', { folder, uid, target }),
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
