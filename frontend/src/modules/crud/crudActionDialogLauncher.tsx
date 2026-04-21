import { createRoot } from 'react-dom/client';
import { useState, type ReactNode } from 'react';
import type { CrudFieldValue } from '@devpablocristo/modules-crud-ui';
import { CrudActionDialog, type CrudActionDialogField } from './CrudActionDialog';
import type { CrudEntityEditorModalBlock, CrudEntityEditorModalSection, CrudEntityEditorModalStat } from './CrudEntityEditorModal';

type CrudFormDialogOptions = {
  title: string;
  subtitle?: string;
  eyebrow?: ReactNode;
  allowEdit?: boolean;
  mediaUrls?: string[];
  mediaFieldId?: string;
  dialogMode?: 'create' | 'update';
  submitLabel?: string;
  cancelLabel?: string;
  editLabel?: string;
  cancelEditLabel?: string;
  closeLabel?: string;
  fields: CrudActionDialogField[];
  blocks?: CrudEntityEditorModalBlock[];
  sections?: CrudEntityEditorModalSection[];
  stats?: CrudEntityEditorModalStat[];
  initialValues?: Record<string, CrudFieldValue>;
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
  onSubmit?: (values: Record<string, CrudFieldValue>) => Promise<void> | void;
};

type CrudTextDialogOptions = {
  title: string;
  subtitle?: string;
  closeLabel?: string;
  textContent: string;
};

function withCrudActionDialog<T>(render: (finish: (result: T) => void) => import('react').ReactNode): Promise<T> {
  if (typeof document === 'undefined') return Promise.resolve(null as T);
  const host = document.createElement('div');
  document.body.appendChild(host);
  const root = createRoot(host);

  return new Promise<T>((resolve) => {
    const finish = (result: T) => {
      root.unmount();
      host.remove();
      resolve(result);
    };
    root.render(render(finish));
  });
}

export function openCrudFormDialog(options: CrudFormDialogOptions): Promise<Record<string, CrudFieldValue> | null> {
  function CrudFormDialogController({ finish }: { finish: (result: Record<string, CrudFieldValue> | null) => void }) {
    const [initialValues, setInitialValues] = useState<Record<string, CrudFieldValue> | undefined>(options.initialValues);
    const [loading, setLoading] = useState(Boolean(options.loading));
    const [error, setError] = useState<string | undefined>(options.error);

    return (
      <CrudActionDialog
        mode="form"
        title={options.title}
        subtitle={options.subtitle}
        eyebrow={options.eyebrow}
        mediaUrls={options.mediaUrls}
        mediaFieldId={options.mediaFieldId}
        dialogMode={options.dialogMode}
        allowEdit={options.allowEdit}
        fields={options.fields}
        blocks={options.blocks}
        sections={options.sections}
        stats={options.stats}
        initialValues={initialValues}
        row={options.row}
        error={error}
        loading={loading}
        loadingLabel={options.loadingLabel}
        disableSubmit={options.disableSubmit}
        editLabel={options.editLabel}
        cancelEditLabel={options.cancelEditLabel}
        closeLabel={options.closeLabel}
        confirmDiscard={options.confirmDiscard}
        archiveAction={
          options.archiveAction
            ? {
                ...options.archiveAction,
                onArchive: async () => {
                  await options.archiveAction?.onArchive();
                  finish(null);
                },
              }
            : undefined
        }
        restoreAction={
          options.restoreAction
            ? {
                ...options.restoreAction,
                onRestore: async () => {
                  await options.restoreAction?.onRestore();
                  finish(null);
                },
              }
            : undefined
        }
        deleteAction={
          options.deleteAction
            ? {
                ...options.deleteAction,
                onDelete: async () => {
                  await options.deleteAction?.onDelete();
                  finish(null);
                },
              }
            : undefined
        }
        submitLabel={options.submitLabel}
        cancelLabel={options.cancelLabel}
        onCancel={() => finish(null)}
        onSubmit={async (values) => {
          if (!options.onSubmit) {
            finish(values);
            return;
          }
          setLoading(true);
          setError(undefined);
          try {
            await options.onSubmit(values);
            setInitialValues(values);
          } catch (submitError) {
            setError(submitError instanceof Error ? submitError.message : 'No se pudo guardar.');
            throw submitError;
          } finally {
            setLoading(false);
          }
        }}
      />
    );
  }

  return withCrudActionDialog<Record<string, CrudFieldValue> | null>((finish) => <CrudFormDialogController finish={finish} />);
}

export function openCrudTextDialog(options: CrudTextDialogOptions): Promise<void> {
  return withCrudActionDialog<void>((finish) => (
    <CrudActionDialog
      mode="text"
      title={options.title}
      subtitle={options.subtitle}
      textContent={options.textContent}
      closeLabel={options.closeLabel}
      onCancel={() => finish()}
    />
  ));
}
