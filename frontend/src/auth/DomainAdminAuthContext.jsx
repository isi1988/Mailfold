import React, { createContext, useContext, useState, useCallback } from 'react';
import { domainAdminApi, getDomainAdminToken, getDomainAdminUser, getDomainAdminDomains, setDomainAdminSession } from '../api/domainAdmin.js';

const Ctx = createContext(null);

export function useDomainAdminAuth() {
  return useContext(Ctx);
}

// DomainAdminAuthProvider owns a domain admin's Mailfold session — distinct
// from both the singleton super-admin's and a webmail mailbox's. There is no
// /me probe on mount (a stored token is treated as authed optimistically, the
// same way webmail's session is); the first API call clears it on 401.
export function DomainAdminAuthProvider({ children }) {
  const [user, setUser] = useState(getDomainAdminUser);
  const [domains, setDomains] = useState(getDomainAdminDomains);
  const [status, setStatus] = useState(() => (getDomainAdminToken() ? 'authed' : 'anon'));

  const applySession = useCallback((token, username, scopedDomains) => {
    setDomainAdminSession(token, username, scopedDomains);
    setUser(username);
    setDomains(scopedDomains || []);
    setStatus('authed');
  }, []);

  const login = useCallback(async (username, password) => {
    const res = await domainAdminApi.login(username, password);
    applySession(res.token, res.user, res.domains);
    return res;
  }, [applySession]);

  const logout = useCallback(async () => {
    try { await domainAdminApi.logout(); } catch { /* best-effort */ }
    setDomainAdminSession('');
    setUser('');
    setDomains([]);
    setStatus('anon');
  }, []);

  const expire = useCallback(() => {
    setDomainAdminSession('');
    setUser('');
    setDomains([]);
    setStatus('anon');
  }, []);

  return (
    <Ctx.Provider value={{ user, domains, status, login, logout, applySession, expire }}>
      {children}
    </Ctx.Provider>
  );
}
