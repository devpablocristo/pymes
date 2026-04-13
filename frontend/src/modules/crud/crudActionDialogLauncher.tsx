import { createRoot } from 'react-dom/client';
import type { ReactNode } from 'react';
import type { CrudFieldValue } from '@devpablocristo/modules-crud-ui';
import { CrudActionDialog, type CrudActionDialogField } from './CrudActionDialog';
import type { CrudEntityEditorModalSection, CrudEntityEditorModalStat } from './CrudEntityEditorModal';

type CrudFormDialogOptions = {
  title: string;
  subtitle?: string;
  eyebrow?: ReactNode;
  mediaUrls?: string[];
  mediaFieldId?: string;
  dialogMode?: 'create' | 'update';
  submitLabel?: string;
  cancelLabel?: string;
  editLabel?: string;
  cancelEditLabel?: string;
  closeLabel?: string;
  fields: CrudActionDialogField[];
  sections?: CrudEntityEditorModalSection[];
  stats?: CrudEntityEditorModalStat[];
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
  return withCrudActionDialog<Record<string, CrudFieldValue> | null>((finish) => (
    <CrudActionDialog
      mode="form"
      title={options.title}
      subtitle={options.subtitle}
      eyebrow={options.eyebrow}
      mediaUrls={options.mediaUrls}
      mediaFieldId={options.mediaFieldId}
      dialogMode={options.dialogMode}
      fields={options.fields}
      sections={options.sections}
      stats={options.stats}
      error={options.error}
      loading={options.loading}
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
      submitLabel={options.submitLabel}
      cancelLabel={options.cancelLabel}
      onCancel={() => finish(null)}
      onSubmit={(values) => finish(values)}
    />
  ));
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
