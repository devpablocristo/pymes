import { createPortal } from 'react-dom';
import { useEffect, type ReactNode } from 'react';
import './CrudEntityModalShell.css';

function cx(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(' ');
}

export type CrudEntityModalShellProps = {
  open: boolean;
  titleId: string;
  ariaLabel?: string;
  children: ReactNode;
  header?: ReactNode;
  footer?: ReactNode;
  variant?: 'modal' | 'page';
  /** Escape y otros cierres “directos” del shell (no backdrop). */
  onRequestClose?: () => void;
  /** Si se define, solo el backdrop lo usa; así se puede suprimir el cierre por clic sin bloquear Escape. */
  onBackdropRequestClose?: () => void;
  disableClose?: boolean;
  closeOnEscape?: boolean;
  closeOnBackdrop?: boolean;
  rootClassName?: string;
  backdropClassName?: string;
  panelClassName?: string;
  headerClassName?: string;
  bodyClassName?: string;
  footerClassName?: string;
  pageToolbar?: ReactNode;
  pageToolbarClassName?: string;
};

export function CrudEntityModalShell({
  open,
  titleId,
  ariaLabel,
  children,
  header,
  footer,
  variant = 'modal',
  onRequestClose,
  onBackdropRequestClose,
  disableClose = false,
  closeOnEscape = variant === 'modal',
  closeOnBackdrop = variant === 'modal',
  rootClassName,
  backdropClassName,
  panelClassName,
  headerClassName,
  bodyClassName,
  footerClassName,
  pageToolbar,
  pageToolbarClassName,
}: CrudEntityModalShellProps) {
  useEffect(() => {
    if (!open || !onRequestClose || !closeOnEscape || disableClose) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        onRequestClose();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [closeOnEscape, disableClose, onRequestClose, open]);

  if (!open) return null;

  const panel = (
    <div
      className={cx('crud-entity-modal-shell__panel', panelClassName)}
      role="dialog"
      aria-modal={variant === 'modal' ? 'true' : 'false'}
      aria-labelledby={header ? titleId : undefined}
      aria-label={header ? undefined : ariaLabel}
    >
      {header ? <div className={cx('crud-entity-modal-shell__header', headerClassName)}>{header}</div> : null}
      <div className={cx('crud-entity-modal-shell__body', bodyClassName)}>{children}</div>
      {footer ? <div className={cx('crud-entity-modal-shell__footer', footerClassName)}>{footer}</div> : null}
    </div>
  );

  if (variant === 'page') {
    return (
      <div className={cx('crud-entity-modal-shell', 'crud-entity-modal-shell--page', rootClassName)}>
        {pageToolbar ? <div className={cx('crud-entity-modal-shell__page-toolbar', pageToolbarClassName)}>{pageToolbar}</div> : null}
        {panel}
      </div>
    );
  }

  if (typeof document === 'undefined') return null;

  return createPortal(
    <div className={cx('crud-entity-modal-shell', rootClassName)} role="presentation">
      {/* div (no button): menos rarezas de foco/hit-testing; el cierre por teclado sigue en Escape */}
      <div
        role="presentation"
        aria-hidden="true"
        className={cx('crud-entity-modal-shell__backdrop', backdropClassName)}
        onClick={(event) => {
          if (disableClose || !closeOnBackdrop) return;
          if (event.target !== event.currentTarget) return;
          const handler = onBackdropRequestClose ?? onRequestClose;
          handler?.();
        }}
      />
      {panel}
    </div>,
    document.body,
  );
}
