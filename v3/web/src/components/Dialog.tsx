import { type FormEvent, type ReactNode, useEffect, useRef } from "react";

type DialogProps = {
  open: boolean;
  title: string;
  eyebrow?: string;
  children: ReactNode;
  footer?: ReactNode;
  onClose: () => void;
  size?: "medium" | "large";
};

export function Dialog({ open, title, eyebrow, children, footer, onClose, size = "medium" }: DialogProps) {
  const ref = useRef<HTMLDialogElement | null>(null);

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      className={`dialog dialog--${size}`}
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
      onClick={(event) => {
        if (event.target === ref.current) onClose();
      }}
    >
      <div className="dialog__surface">
        <header className="dialog__header">
          <div>
            {eyebrow ? <p className="eyebrow">{eyebrow}</p> : null}
            <h2>{title}</h2>
          </div>
          <button type="button" className="icon-button" aria-label="Cerrar" onClick={onClose}>
            ×
          </button>
        </header>
        <div className="dialog__body">{children}</div>
        {footer ? <footer className="dialog__footer">{footer}</footer> : null}
      </div>
    </dialog>
  );
}

type FormDialogProps = DialogProps & {
  formId: string;
  submitLabel: string;
  pending?: boolean;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
};

export function FormDialog({
  formId,
  submitLabel,
  pending,
  onSubmit,
  children,
  ...dialogProps
}: FormDialogProps) {
  return (
    <Dialog
      {...dialogProps}
      footer={
        <>
          <button type="button" className="button button--quiet" onClick={dialogProps.onClose}>
            Volver
          </button>
          <button type="submit" className="button button--primary" form={formId} disabled={pending}>
            {pending ? "Guardando…" : submitLabel}
          </button>
        </>
      }
    >
      <form id={formId} onSubmit={onSubmit}>
        {children}
      </form>
    </Dialog>
  );
}
