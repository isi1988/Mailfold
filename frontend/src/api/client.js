// Thin fetch wrapper for the Mailfold backend. The SPA is served same-origin in
// production and proxied in dev, so paths are root-relative ("/api/..."). Auth is
// a bearer token kept in localStorage and sent on every request.

const TOKEN_KEY = 'mailfold.token';

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || '';
}

export function setToken(token) {
  if (token) localStorage.setItem(TOKEN_KEY, token);
  else localStorage.removeItem(TOKEN_KEY);
}

export class ApiError extends Error {
  constructor(status, message, body) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.body = body;
  }
}

// Subscribers notified when a request is rejected with 401, so the app can drop
// to the login screen from anywhere without prop-drilling.
const unauthorizedHandlers = new Set();
export function onUnauthorized(fn) {
  unauthorizedHandlers.add(fn);
  return () => unauthorizedHandlers.delete(fn);
}

async function request(method, path, { body, headers, signal } = {}) {
  const h = { Accept: 'application/json', ...headers };
  const token = getToken();
  if (token) h.Authorization = 'Bearer ' + token;

  let payload;
  if (body !== undefined && body !== null) {
    h['Content-Type'] = 'application/json';
    payload = JSON.stringify(body);
  }

  const res = await fetch(path, { method, headers: h, body: payload, signal });
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
    if (res.status === 401) unauthorizedHandlers.forEach(fn => fn());
    const message = (data && data.error) || res.statusText || 'request failed';
    throw new ApiError(res.status, message, data);
  }
  return data;
}

export const api = {
  get: (path, opts) => request('GET', path, opts),
  post: (path, body, opts) => request('POST', path, { ...opts, body }),
  put: (path, body, opts) => request('PUT', path, { ...opts, body }),
  del: (path, body, opts) => request('DELETE', path, { ...opts, body }),
};
