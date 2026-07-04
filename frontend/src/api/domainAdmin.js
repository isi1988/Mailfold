// Domain admin talks to /api/domain-admin/* and /api/auth/domain-admin/* with
// its OWN bearer token — a session distinct from both the admin panel's and a
// webmail mailbox's — kept separately in localStorage. Unlike webmail, a
// domain admin only ever has one active session at a time (no multi-account
// switcher), so this mirrors client.js's simplicity rather than webmail.js's.

const TOKEN_KEY = 'mailfold.domainAdmin.token';
const USER_KEY = 'mailfold.domainAdmin.user';
const DOMAINS_KEY = 'mailfold.domainAdmin.domains';

export function getDomainAdminToken() {
  return localStorage.getItem(TOKEN_KEY) || '';
}
export function getDomainAdminUser() {
  return localStorage.getItem(USER_KEY) || '';
}
export function getDomainAdminDomains() {
  try {
    return JSON.parse(localStorage.getItem(DOMAINS_KEY) || '[]');
  } catch {
    return [];
  }
}
export function setDomainAdminSession(token, user, domains) {
  if (token) {
    localStorage.setItem(TOKEN_KEY, token);
    localStorage.setItem(USER_KEY, user || '');
    localStorage.setItem(DOMAINS_KEY, JSON.stringify(domains || []));
  } else {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
    localStorage.removeItem(DOMAINS_KEY);
  }
}

async function req(method, path, body) {
  const headers = { Accept: 'application/json' };
  const token = getDomainAdminToken();
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

export const domainAdminApi = {
  login: (user, password) => req('POST', '/api/auth/domain-admin/login', { user, password }),
  logout: () => req('POST', '/api/auth/domain-admin/logout'),
  me: () => req('GET', '/api/auth/domain-admin/me'),
  ssoProviders: {
    list: () => req('GET', '/api/domain-admin/sso-providers'),
    create: provider => req('POST', '/api/domain-admin/sso-providers', provider),
    update: (id, provider) => req('PUT', '/api/domain-admin/sso-providers', { id, ...provider }),
    del: id => req('DELETE', '/api/domain-admin/sso-providers', { id: String(id) }),
  },
};
