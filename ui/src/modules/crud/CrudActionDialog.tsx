import type { CrudFieldValue } from '@devpablocristo/modules-crud-ui';
import type { ReactNode } from 'react';
import {
  CrudEntityEditorModal,
  type CrudEntityEditorModalBlock,
  type CrudEntityEditorModalField,
  type CrudEntityEditorModalSection,
  type CrudEntityEditorModalStat,
} from './CrudEntityEditorModal';
import { CrudEntityModalShell } from './CrudEntityModalShell';
import './CrudActionDialog.css';

export type CrudActionDialogField = CrudEntityEditorModalField;

type CrudActionDialogBaseProps = {
  title: string;
  subtitle?: string;
  eyebrow?: ReactNode;
  mediaUrls?: string[];
  mediaFieldId?: string;
  cancelLabel?: string;
  onCancel: () => void;
};

type CrudActionDialogFormProps = CrudActionDialogBaseProps & {
  mode: 'form';
  allowEdit?: boolean;
  fields: CrudActionDialogField[];
  sections?: CrudEntityEditorModalSection[];
  stats?: CrudEntityEditorModalStat[];
  error?: string;
  loading?: boolean;
  loadingLabel?: string;
  disableSubmit?: boolean;
  dialogMode?: 'create' | 'update';
  editLabel?: string;
  cancelEditLabel?: string;
  closeLabel?: string;
  confirmDiscard?: {
    title: string;
    description: string;
    confirmLabel?: string;
    cancelLabel?: string;
  };
  archiveAction?: {
    label?: string;
    busyLabel?: string;
    confirm?: {
      title: string;
      description: string;
      confirmLabel?: string;
      cancelLabel?: string;
    };
    onArchive: () => Promise<void> | void;
  };
  restoreAction?: {
    label?: string;
    busyLabel?: string;
    confirm?: {
      title: string;
      description: string;
      confirmLabel?: string;
      cancelLabel?: string;
    };
    onRestore: () => Promise<void> | void;
  };
  deleteAction?: {
    label?: string;
    busyLabel?: string;
    confirm?: {
      title: string;
      description: string;
      confirmLabel?: string;
      cancelLabel?: string;
    };
    onDelete: () => Promise<void> | void;
  };
  blocks?: CrudEntityEditorModalBlock[];
  submitLabel?: string;
  initialValues?: Record<string, CrudFieldValue>;
  row?: unknown;
  onSubmit: (values: Record<string, CrudFieldValue>) => void;
};

type CrudActionDialogTextProps = CrudActionDialogBaseProps & {
  mode: 'text';
  textContent: string;
  closeLabel?: string;
};

export type CrudActionDialogProps = CrudActionDialogFormProps | CrudActionDialogTextProps;

export function CrudActionDialog(props: CrudActionDialogProps) {
  if (props.mode === 'form') {
    return (
      <CrudEntityEditorModal
        open
        title={props.title}
        subtitle={props.subtitle}
        eyebrow={props.eyebrow}
        mediaUrls={props.mediaUrls}
        mediaFieldId={props.mediaFieldId}
        mode={props.dialogMode}
        allowEdit={props.allowEdit}
        editLabel={props.editLabel}
        cancelEditLabel={props.cancelEditLabel}
        closeLabel={props.closeLabel}
        fields={props.fields}
        blocks={props.blocks}
        sections={props.sections}
        stats={props.stats}
        initialValues={props.initialValues}
        row={props.row}
        error={props.error}
        loading={props.loading}
        loadingLabel={props.loadingLabel}
        disableSubmit={props.disableSubmit}
        confirmDiscard={props.confirmDiscard}
        archiveAction={props.archiveAction}
        restoreAction={props.restoreAction}
        deleteAction={props.deleteAction}
        submitLabel={props.submitLabel}
        cancelLabel={props.cancelLabel}
        onCancel={props.onCancel}
        onSubmit={props.onSubmit}
      />
    );
  }

  return (
    <CrudEntityModalShell
      open
      titleId="crud-action-dialog-title"
      onRequestClose={props.onCancel}
      rootClassName="crud-action-dialog-root"
      backdropClassName="crud-action-dialog__backdrop"
      panelClassName="crud-action-dialog"
      headerClassName="crud-action-dialog__header"
      bodyClassName="crud-action-dialog__body"
      footerClassName="crud-action-dialog__footer"
      header={
        <div className="crud-action-dialog__title-block">
          <h2 className="crud-action-dialog__title" id="crud-action-dialog-title">
            {props.title}
          </h2>
          {props.subtitle ? <p className="crud-action-dialog__subtitle">{props.subtitle}</p> : null}
        </div>
      }
      footer={
        <button type="button" className="btn btn-primary" onClick={props.onCancel}>
          {props.closeLabel ?? 'Cerrar'}
        </button>
      }
    >
      <pre className="crud-action-dialog__text">{props.textContent}</pre>
    </CrudEntityModalShell>
  );
}
