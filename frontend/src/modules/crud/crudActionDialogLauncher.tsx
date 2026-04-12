import { createRoot } from 'react-dom/client';
import type { CrudFieldValue } from '@devpablocristo/modules-crud-ui';
import { CrudActionDialog, type CrudActionDialogField } from './CrudActionDialog';

type CrudFormDialogOptions = {
  title: string;
  subtitle?: string;
  submitLabel?: string;
  cancelLabel?: string;
  fields: CrudActionDialogField[];
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
      fields={options.fields}
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
