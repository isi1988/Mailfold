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
  // justAdded/temporary back the "link this mailbox to your admin account?"
  // prompt: an admin opening a mailbox's webmail they hadn't opened before
  // gets asked once, right after login. justAdded names the account the
  // prompt is currently pending for; temporary holds the ones the admin
  // chose "just viewing" for, so they can be dropped again on exit.
  const [justAdded, setJustAdded] = useState(null);
  const [temporary, setTemporary] = useState(() => new Set());

  // applySession commits a token (from wm.login or the unified login) as an
  // account and makes it active.
  const applySession = useCallback((token, mailbox, external = false) => {
    const list = addAccount(mailbox, token, external);
    setAccounts(list);
    setEmail(mailbox);
    setStatus('authed');
  }, []);

  // commitLoginResult applies a successful login/2FA-verify response (both
  // shapes carry {token, email}) as a session, flagging a genuinely new
  // account via justAdded so the caller can offer the link-confirmation
  // prompt.
  const commitLoginResult = useCallback((res, mailbox) => {
    const finalEmail = res.email || mailbox;
    const alreadyLinked = getAccounts().some(a => a.email === finalEmail);
    applySession(res.token, finalEmail);
    if (!alreadyLinked) setJustAdded(finalEmail);
  }, [applySession]);

  // login adds (and switches to) another mailbox account — unless the
  // account has two-factor auth enabled, in which case it returns
  // {needs_2fa, pending_token} instead of committing anything, and the
  // caller must complete the second factor via verifyLogin2FA.
  const login = useCallback(async (mailbox, password) => {
    const res = await wm.login(mailbox, password);
    if (res.needs_2fa) return res;
    commitLoginResult(res, mailbox);
    return res;
  }, [commitLoginResult]);

  // verifyLogin2FA redeems a pending login (from login() above) with a
  // TOTP/recovery code and commits the resulting session.
  const verifyLogin2FA = useCallback(async (pendingToken, code, mailbox) => {
    const res = await wm.totp.verify(pendingToken, code);
    commitLoginResult(res, mailbox);
    return res;
  }, [commitLoginResult]);

  const clearJustAdded = useCallback(() => setJustAdded(null), []);

  // markTemporary records that the just-added account should be forgotten
  // (not kept as a permanently linked account) once the viewer leaves.
  const markTemporary = useCallback(mailbox => {
    setTemporary(cur => new Set(cur).add(mailbox));
    setJustAdded(null);
  }, []);

  // cleanupTemporary drops every account marked temporary — called when an
  // admin who chose "just viewing" navigates away from webmail.
  const cleanupTemporary = useCallback(() => {
    if (temporary.size === 0) return;
    let list = getAccounts();
    temporary.forEach(mailbox => { list = removeAcc(mailbox); });
    setTemporary(new Set());
    setAccounts(list);
    setEmail(getActiveEmail());
    setStatus(list.length ? 'authed' : 'anon');
  }, [temporary]);

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
    <Ctx.Provider value={{
      email, accounts, status, login, verifyLogin2FA, logout, applySession, switchAccount, removeAccount: dropAccount, expire,
      justAdded, clearJustAdded, markTemporary, cleanupTemporary,
    }}>
      {children}
    </Ctx.Provider>
  );
}
