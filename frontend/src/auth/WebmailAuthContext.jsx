import React, { createContext, useContext, useState, useCallback } from 'react';
import { wm, getAccounts, getActiveEmail, addAccount, removeAccount as removeAcc, setActiveAccount } from '../api/webmail.js';

const Ctx = createContext(null);

export function useWebmailAuth() {
  return useContext(Ctx);
}

// WebmailAuthProvider owns the mailbox (webmail) sessions, kept separate from the
// admin session. A user can hold several accounts at once and switch between
// them. There is no /me probe for webmail, so a stored token is treated as authed
// optimistically; the first API call clears it on 401.
export function WebmailAuthProvider({ children }) {
  const [accounts, setAccounts] = useState(getAccounts);
  const [email, setEmail] = useState(getActiveEmail);
  const [status, setStatus] = useState(() => (getAccounts().length ? 'authed' : 'anon'));

  // applySession commits a token (from wm.login or the unified login) as an
  // account and makes it active.
  const applySession = useCallback((token, mailbox, external = false) => {
    const list = addAccount(mailbox, token, external);
    setAccounts(list);
    setEmail(mailbox);
    setStatus('authed');
  }, []);

  // login adds (and switches to) another mailbox account.
  const login = useCallback(async (mailbox, password) => {
    const res = await wm.login(mailbox, password);
    applySession(res.token, res.email || mailbox);
    return res;
  }, [applySession]);

  const switchAccount = useCallback(mailbox => {
    setActiveAccount(mailbox);
    setEmail(mailbox);
  }, []);

  // dropAccount removes one account; if others remain the session stays on the
  // next one, otherwise the webmail returns to the sign-in screen.
  const dropAccount = useCallback(mailbox => {
    const list = removeAcc(mailbox);
    setAccounts(list);
    setEmail(getActiveEmail());
    setStatus(list.length ? 'authed' : 'anon');
  }, []);

  const logout = useCallback(async () => {
    try { await wm.logout(); } catch { /* best-effort */ }
    dropAccount(getActiveEmail());
  }, [dropAccount]);

  // expire is called on a 401 for the active account: drop just that one.
  const expire = useCallback(() => {
    dropAccount(getActiveEmail());
  }, [dropAccount]);

  return (
    <Ctx.Provider value={{ email, accounts, status, login, logout, applySession, switchAccount, removeAccount: dropAccount, expire }}>
      {children}
    </Ctx.Provider>
  );
}
