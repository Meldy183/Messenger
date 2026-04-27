import React, { createContext, useCallback, useContext, useRef, useState } from 'react';

export type ToastType = 'error' | 'success' | 'info';

interface ToastItem {
  id: number;
  type: ToastType;
  message: string;
  hint?: string;
}

interface ToastContextValue {
  show: (message: string, type?: ToastType, hint?: string) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);
let _id = 0;

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const timers = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map());

  const dismiss = useCallback((id: number) => {
    setToasts(prev => prev.filter(t => t.id !== id));
    const t = timers.current.get(id);
    if (t) { clearTimeout(t); timers.current.delete(id); }
  }, []);

  const show = useCallback((message: string, type: ToastType = 'error', hint?: string) => {
    const id = ++_id;
    setToasts(prev => [...prev.slice(-4), { id, type, message, hint }]);
    timers.current.set(id, setTimeout(() => dismiss(id), 6000));
  }, [dismiss]);

  return (
    <ToastContext.Provider value={{ show }}>
      {children}
      <div className="toast-stack" aria-live="assertive" aria-atomic="false">
        {toasts.map(t => (
          <div key={t.id} className={`toast toast-${t.type}`} role="alert">
            <span className="toast-icon" aria-hidden="true">
              {t.type === 'error' ? '⚠' : t.type === 'success' ? '✓' : 'ℹ'}
            </span>
            <div className="toast-body">
              <span className="toast-message">{t.message}</span>
              {t.hint && <span className="toast-hint">{t.hint}</span>}
            </div>
            <button className="toast-close" onClick={() => dismiss(t.id)} aria-label="Dismiss">✕</button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error('useToast must be used within ToastProvider');
  return ctx;
}
