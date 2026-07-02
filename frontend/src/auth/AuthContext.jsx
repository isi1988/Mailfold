import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';
import { api, getToken, setToken, onUnauthorized } from '../api/client.js';

const AuthCtx = createContext(null);

export function useAuth() {
  return useContext(AuthCtx);
}

// AuthProvider owns the admin session: it restores it on load (validating the
// stored token against /api/auth/me), exposes login/logout, and flips to the
// anonymous state whenever any request returns 401.
export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [status, setStatus] = useState('loading'); // loading | authed | anon

  const refresh = useCallback(async () => {
    if (!getToken()) {
      setStatus('anon');
      return;
    }
    try {
      const me = await api.get('/api/auth/me');
      setUser(me.user);
      setStatus('authed');
    } catch {
      setToken('');
      setUser(null);
      setStatus('anon');
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  useEffect(() => {
    return onUnauthorized(() => {
      setToken('');
      setUser(null);
      setStatus('anon');
    });
  }, []);

  const login = useCallback(async (userName, password) => {
    const res = await api.post('/api/auth/login', { user: userName, password });
    setToken(res.token);
    setUser(res.user);
    setStatus('authed');
    return res;
  }, []);

  const logout = useCallback(async () => {
    try {
      await api.post('/api/auth/logout');
    } catch {
      /* best-effort; local state is cleared regardless */
    }
    setToken('');
    setUser(null);
    setStatus('anon');
  }, []);

  return (
    <AuthCtx.Provider value={{ user, status, login, logout, refresh }}>
      {children}
    </AuthCtx.Provider>
  );
}
