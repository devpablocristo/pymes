import type { ReactNode } from 'react';
import { CrudEntityModalShell } from './CrudEntityModalShell';
import './CrudEntityDetailModal.css';

export type CrudEntityDetailField = {
  id: string;
  label: ReactNode;
  value: ReactNode;
  fullWidth?: boolean;
  valueClassName?: string;
};

export type CrudEntityDetailModalProps = {
  open: boolean;
  titleId: string;
  title: ReactNode;
  subtitle?: ReactNode;
  onClose: () => void;
  closeLabel?: string;
  media?: ReactNode;
  fields?: CrudEntityDetailField[];
  children?: ReactNode;
  error?: ReactNode;
  loading?: boolean;
  loadingLabel?: ReactNode;
  rootClassName?: string;
  backdropClassName?: string;
  panelClassName?: string;
  headerClassName?: string;
  bodyClassName?: string;
  footerClassName?: string;
};

function cx(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(' ');
}

export function CrudEntityDetailModal({
  open,
  titleId,
  title,
  subtitle,
  onClose,
  closeLabel = 'Cerrar',
  media,
  fields,
  children,
  error,
  loading = false,
  loadingLabel = 'Cargando…',
  rootClassName,
  backdropClassName,
  panelClassName,
  headerClassName,
  bodyClassName,
  footerClassName,
}: CrudEntityDetailModalProps) {
  return (
    <CrudEntityModalShell
      open={open}
      titleId={titleId}
      onRequestClose={onClose}
      rootClassName={cx('crud-entity-detail-modal-root', rootClassName)}
      backdropClassName={cx('crud-entity-detail-modal__backdrop', backdropClassName)}
      panelClassName={cx('crud-entity-detail-modal', panelClassName)}
      headerClassName={cx('crud-entity-detail-modal__header', headerClassName)}
      bodyClassName={cx('crud-entity-detail-modal__body', bodyClassName)}
      footerClassName={cx('crud-entity-detail-modal__footer', footerClassName)}
      header={
        <>
          <div className="crud-entity-detail-modal__title-block">
            <h2 id={titleId} className="crud-entity-detail-modal__title">
              {title}
            </h2>
            {subtitle ? <p className="crud-entity-detail-modal__subtitle">{subtitle}</p> : null}
          </div>
          <button type="button" className="crud-entity-detail-modal__close" onClick={onClose}>
            {closeLabel}
          </button>
        </>
      }
      footer={
        <button type="button" className="btn btn-primary" onClick={onClose}>
          {closeLabel}
        </button>
      }
    >
      {error ? (
        <p className="crud-entity-detail-modal__error">{error}</p>
      ) : loading ? (
        <p className="crud-entity-detail-modal__loading">{loadingLabel}</p>
      ) : (
        <>
          {media ? <div className="crud-entity-detail-modal__media">{media}</div> : null}
          {fields?.length ? (
            <dl className="crud-entity-detail-modal__fields">
              {fields.map((field) => (
                <div
                  key={field.id}
                  className={cx('crud-entity-detail-modal__row', field.fullWidth && 'crud-entity-detail-modal__row--full')}
                >
                  <dt>{field.label}</dt>
                  <dd className={field.valueClassName}>{field.value}</dd>
                </div>
              ))}
            </dl>
          ) : null}
          {children}
        </>
      )}
    </CrudEntityModalShell>
  );
}
