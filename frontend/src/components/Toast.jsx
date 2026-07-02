import React, { createContext, useContext, useState, useCallback, useRef } from 'react';
import { Toaster } from '../ds/components/organisms/Toaster.jsx';
import { Toast } from '../ds/components/molecules/Toast.jsx';

const ToastCtx = createContext(null);

export function useToast() {
  return useContext(ToastCtx);
}

// ToastProvider owns a small auto-dismissing toast queue rendered once at the app
// root, so any page can surface success/error feedback via useToast().
export function ToastProvider({ children }) {
  const [toasts, setToasts] = useState([]);
  const idRef = useRef(0);

  const dismiss = useCallback(id => {
    setToasts(ts => ts.filter(t => t.id !== id));
  }, []);

  const toast = useCallback((title, msg) => {
    const id = (idRef.current += 1);
    setToasts(ts => [...ts, { id, title, msg }]);
    setTimeout(() => dismiss(id), 4000);
  }, [dismiss]);

  return (
    <ToastCtx.Provider value={{ toast }}>
      {children}
      {toasts.length > 0 && (
        <Toaster>
          {toasts.map(t => (
            <Toast key={t.id} title={t.title} msg={t.msg} onDismiss={() => dismiss(t.id)} />
          ))}
        </Toaster>
      )}
    </ToastCtx.Provider>
  );
}
