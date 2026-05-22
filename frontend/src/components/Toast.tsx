import { useToast } from "@/hooks/useToast";

export function ToastRegion() {
  const { toasts, dismissToast } = useToast();
  return (
    <div className="toast-region" aria-live="polite">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className="toast"
          role="alert"
          onClick={() => dismissToast(toast.id)}
        >
          <div className="toast__icon" aria-hidden="true">i</div>
          <div>
            <div className="toast__title">{toast.title}</div>
            <div className="toast__body">{toast.body}</div>
          </div>
        </div>
      ))}
    </div>
  );
}
