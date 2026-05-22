import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

export type Toast = {
  id: string;
  title: string;
  body: string;
  durationMs: number;
};

type ShowToastInput = {
  title: string;
  body: string;
  durationMs?: number;
};

type ToastContextValue = {
  toasts: Toast[];
  showToast: (input: ShowToastInput) => void;
  dismissToast: (id: string) => void;
};

const ToastContext = createContext<ToastContextValue | null>(null);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const dismissToast = useCallback((id: string) => {
    setToasts((current) => current.filter((t) => t.id !== id));
  }, []);

  const showToast = useCallback(({ title, body, durationMs = 5000 }: ShowToastInput) => {
    const id = crypto.randomUUID();
    setToasts((current) => [...current, { id, title, body, durationMs }]);
  }, []);

  // Auto-dismiss expired toasts.
  useEffect(() => {
    if (toasts.length === 0) return;
    const timers = toasts.map((toast) =>
      window.setTimeout(() => dismissToast(toast.id), toast.durationMs),
    );
    return () => {
      timers.forEach((t) => window.clearTimeout(t));
    };
  }, [toasts, dismissToast]);

  const value = useMemo(() => ({ toasts, showToast, dismissToast }), [toasts, showToast, dismissToast]);
  return <ToastContext.Provider value={value}>{children}</ToastContext.Provider>;
}

export function useToast() {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within ToastProvider");
  return ctx;
}
