import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { subscribeToast, TOAST_ERROR } from '../toastBus.js';

const ToastContext = createContext(null);
const DEFAULT_DURATION = 4000;

function buildId() {
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

export function ToastProvider({ children }) {
  const [toasts, setToasts] = useState([]);

  const removeToast = useCallback((id) => {
    setToasts((prev) => prev.filter((toast) => toast.id !== id));
  }, []);

  const addToast = useCallback(
    (toast) => {
      if (!toast?.message) {
        return;
      }
      const id = toast.id || buildId();
      const entry = {
        id,
        message: toast.message,
        type: toast.type || TOAST_ERROR,
      };
      setToasts((prev) => [...prev, entry]);
      const duration = Number.isFinite(toast.duration) ? toast.duration : DEFAULT_DURATION;
      if (duration > 0) {
        window.setTimeout(() => {
          removeToast(id);
        }, duration);
      }
    },
    [removeToast],
  );

  useEffect(() => {
    return subscribeToast(addToast);
  }, [addToast]);

  const value = useMemo(() => ({ addToast, removeToast }), [addToast, removeToast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="toast-container" role="status" aria-live="polite">
        {toasts.map((toast) => (
          <div key={toast.id} className={`toast toast--${toast.type}`}>
            <span>{toast.message}</span>
            <button className="toast-close" type="button" onClick={() => removeToast(toast.id)}>
              ×
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  return useContext(ToastContext);
}
