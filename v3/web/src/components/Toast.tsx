import { useEffect } from "react";

export type ToastMessage = {
  id: string;
  tone: "success" | "error" | "info";
  text: string;
};

export function Toast({ message, onDismiss }: { message: ToastMessage | null; onDismiss: () => void }) {
  useEffect(() => {
    if (!message) return;
    const timeout = window.setTimeout(onDismiss, 5_000);
    return () => window.clearTimeout(timeout);
  }, [message, onDismiss]);

  if (!message) return null;
  return (
    <div className={`toast toast--${message.tone}`} role={message.tone === "error" ? "alert" : "status"}>
      <span>{message.text}</span>
      <button type="button" aria-label="Cerrar mensaje" onClick={onDismiss}>
        ×
      </button>
    </div>
  );
}
