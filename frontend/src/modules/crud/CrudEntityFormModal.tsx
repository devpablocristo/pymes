import {
  CrudEntityEditorModal,
  type CrudEntityEditorModalBlock,
  type CrudEntityEditorModalField,
  type CrudEntityEditorModalSection,
  type CrudEntityEditorModalStat,
} from './CrudEntityEditorModal';

export type CrudEntityFormModalField = CrudEntityEditorModalField;

export type CrudEntityFormModalProps = {
  open: boolean;
  title: string;
  subtitle?: string;
  eyebrow?: import('react').ReactNode;
  mediaUrls?: string[];
  mediaFieldId?: string;
  mode?: 'create' | 'update';
  cancelLabel?: string;
  submitLabel?: string;
  editLabel?: string;
  cancelEditLabel?: string;
  closeLabel?: string;
  fields: CrudEntityFormModalField[];
  blocks?: CrudEntityEditorModalBlock[];
  sections?: CrudEntityEditorModalSection[];
  stats?: CrudEntityEditorModalStat[];
  initialValues?: Record<string, import('@devpablocristo/modules-crud-ui').CrudFieldValue>;
  row?: unknown;
  error?: string;
  loading?: boolean;
  loadingLabel?: string;
  disableSubmit?: boolean;
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
  onCancel: () => void;
  onSubmit: (values: Record<string, import('@devpablocristo/modules-crud-ui').CrudFieldValue>) => void;
};

export function CrudEntityFormModal({
  open,
  title,
  subtitle,
  eyebrow,
  mediaUrls,
  mediaFieldId,
  mode,
  cancelLabel,
  submitLabel,
  editLabel,
  cancelEditLabel,
  closeLabel,
  fields,
  blocks,
  sections,
  stats,
  initialValues,
  row,
  error,
  loading,
  loadingLabel,
  disableSubmit,
  confirmDiscard,
  archiveAction,
  onCancel,
  onSubmit,
}: CrudEntityFormModalProps) {
  return (
    <CrudEntityEditorModal
      open={open}
      title={title}
      subtitle={subtitle}
      eyebrow={eyebrow}
      mediaUrls={mediaUrls}
      mediaFieldId={mediaFieldId}
      mode={mode}
      cancelLabel={cancelLabel}
      submitLabel={submitLabel}
      editLabel={editLabel}
      cancelEditLabel={cancelEditLabel}
      closeLabel={closeLabel}
      fields={fields}
      blocks={blocks}
      sections={sections}
      stats={stats}
      initialValues={initialValues}
      row={row}
      error={error}
      loading={loading}
      loadingLabel={loadingLabel}
      disableSubmit={disableSubmit}
      confirmDiscard={confirmDiscard}
      archiveAction={archiveAction}
      onCancel={onCancel}
      onSubmit={onSubmit}
    />
  );
}
