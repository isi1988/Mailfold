import React, { useEffect, useState } from 'react';
import { Button } from '../ds/components/atoms/Button.jsx';
import { wm } from '../api/webmail.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';

/**
 * Fixed bottom-center bar shown right after a message is sent, while it still
 * sits inside the server's short "undo" window (see ComposeModal.send()).
 * Counts down live to scheduledAt and offers an Undo button that cancels the
 * scheduled send. Auto-dismisses itself once the countdown reaches zero.
 *
 *   info      { id, scheduledAt } — scheduledAt is an RFC3339 string
 *   onDismiss () => void
 */
export function UndoSendBar({ info, onDismiss }) {
  const t = useT();
  const { toast } = useToast();
  const [secondsLeft, setSecondsLeft] = useState(() => remaining(info.scheduledAt));
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const id = setInterval(() => {
      setSecondsLeft(remaining(info.scheduledAt));
    }, 1000);
    return () => clearInterval(id);
  }, [info.scheduledAt]);

  useEffect(() => {
    if (secondsLeft <= 0) onDismiss();
  }, [secondsLeft, onDismiss]);

  async function undo() {
    if (busy) return;
    setBusy(true);
    try {
      await wm.cancelScheduled(info.id);
      toast(t('webmail.undoSend.undone'));
    } catch {
      toast(t('webmail.undoSend.tooLate'));
    } finally {
      setBusy(false);
      onDismiss();
    }
  }

  if (secondsLeft <= 0) return null;

  return (
    <div
      style={{
        position: 'fixed', left: '50%', bottom: 28, transform: 'translateX(-50%)', zIndex: 60,
        display: 'flex', alignItems: 'center', gap: 14, padding: '10px 14px 10px 18px',
        background: 'var(--ink-strong)', color: '#fff', borderRadius: 12, boxShadow: 'var(--shadow-modal)',
        fontSize: 13.5,
      }}
    >
      <span>{t('webmail.undoSend.sendingIn', { count: secondsLeft })}</span>
      <Button variant="secondary" size="sm" onClick={undo} disabled={busy}>{t('webmail.undoSend.undo')}</Button>
    </div>
  );
}

function remaining(scheduledAt) {
  const ms = new Date(scheduledAt).getTime() - Date.now();
  return Math.max(0, Math.ceil(ms / 1000));
}
