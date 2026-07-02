import React, { createContext, useContext, useState, useCallback } from 'react';
import { wm, getWebmailToken, setWebmailToken, getWebmailEmail, setWebmailEmail } from '../api/webmail.js';

const Ctx = createContext(null);

export function useWebmailAuth() {
  return useContext(Ctx);
}

// WebmailAuthProvider owns the mailbox (webmail) session, kept separate from the
// admin session. There is no /me probe for webmail, so a stored token is treated
// as authed optimistically; the first API call clears it on 401.
export function WebmailAuthProvider({ children }) {
  const [email, setEmail] = useState(getWebmailEmail);
  const [status, setStatus] = useState(() => (getWebmailToken() ? 'authed' : 'anon'));

  // applySession commits a token obtained elsewhere (e.g. the unified login).
  const applySession = useCallback((token, mailbox) => {
    setWebmailToken(token);
    setWebmailEmail(mailbox);
    setEmail(mailbox);
    setStatus('authed');
  }, []);

  const login = useCallback(async (mailbox, password) => {
    const res = await wm.login(mailbox, password);
    applySession(res.token, res.email || mailbox);
    return res;
  }, [applySession]);

  const logout = useCallback(async () => {
    try {
      await wm.logout();
    } catch {
      /* best-effort */
    }
    setWebmailToken('');
    setWebmailEmail('');
    setEmail('');
    setStatus('anon');
  }, []);

  const expire = useCallback(() => {
    setWebmailToken('');
    setStatus('anon');
  }, []);

  return (
    <Ctx.Provider value={{ email, status, login, logout, applySession, expire }}>
      {children}
    </Ctx.Provider>
  );
}
