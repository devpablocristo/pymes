import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { createPortal } from 'react-dom';
import type { CrudFieldValue } from '@devpablocristo/modules-crud-ui';
import './CrudActionDialog.css';

export type CrudActionDialogField = {
  id: string;
  label: string;
  type?: 'text' | 'email' | 'tel' | 'number' | 'textarea' | 'datetime-local' | 'select' | 'checkbox';
  placeholder?: string;
  required?: boolean;
  defaultValue?: CrudFieldValue;
  min?: number;
  step?: number | 'any';
  rows?: number;
  options?: Array<{ label: string; value: string }>;
};

type CrudActionDialogBaseProps = {
  title: string;
  subtitle?: string;
  cancelLabel?: string;
  onCancel: () => void;
};

type CrudActionDialogFormProps = CrudActionDialogBaseProps & {
  mode: 'form';
  fields: CrudActionDialogField[];
  submitLabel?: string;
  onSubmit: (values: Record<string, CrudFieldValue>) => void;
};

type CrudActionDialogTextProps = CrudActionDialogBaseProps & {
  mode: 'text';
  textContent: string;
  closeLabel?: string;
};

export type CrudActionDialogProps = CrudActionDialogFormProps | CrudActionDialogTextProps;

export function CrudActionDialog(props: CrudActionDialogProps) {
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') props.onCancel();
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [props]);

  const initialValues = useMemo(
    () =>
      props.mode === 'form'
        ? Object.fromEntries(props.fields.map((field) => [field.id, field.defaultValue ?? (field.type === 'checkbox' ? false : '')]))
        : {},
    [props],
  );
  const [values, setValues] = useState<Record<string, CrudFieldValue>>(initialValues);

  useEffect(() => {
    setValues(initialValues);
  }, [initialValues]);

  if (!mounted || typeof document === 'undefined') return null;

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (props.mode !== 'form') return;
    const form = event.currentTarget;
    if (!form.reportValidity()) return;
    props.onSubmit(values);
  };

  const body = (
    <div className="crud-action-dialog-root" role="presentation">
      <button className="crud-action-dialog__backdrop" type="button" aria-label="Cerrar" onClick={props.onCancel} />
      <div className="crud-action-dialog" role="dialog" aria-modal="true" aria-labelledby="crud-action-dialog-title">
        <header className="crud-action-dialog__header">
          <div className="crud-action-dialog__title-block">
            <h2 className="crud-action-dialog__title" id="crud-action-dialog-title">
              {props.title}
            </h2>
            {props.subtitle ? <p className="crud-action-dialog__subtitle">{props.subtitle}</p> : null}
          </div>
        </header>

        {props.mode === 'form' ? (
          <form className="crud-action-dialog__form" onSubmit={handleSubmit}>
            <div className="crud-action-dialog__body">
              {props.fields.map((field) => (
                <label key={field.id} className="crud-action-dialog__field">
                  <span>{field.label}</span>
                  {field.type === 'textarea' ? (
                    <textarea
                      value={String(values[field.id] ?? '')}
                      onChange={(event) => setValues((current) => ({ ...current, [field.id]: event.target.value }))}
                      placeholder={field.placeholder}
                      required={field.required}
                      rows={field.rows ?? 4}
                    />
                  ) : field.type === 'select' ? (
                    <select
                      value={String(values[field.id] ?? '')}
                      onChange={(event) => setValues((current) => ({ ...current, [field.id]: event.target.value }))}
                      required={field.required}
                    >
                      <option value="">{field.placeholder ?? 'Seleccionar...'}</option>
                      {(field.options ?? []).map((option) => (
                        <option key={option.value} value={option.value}>
                          {option.label}
                        </option>
                      ))}
                    </select>
                  ) : field.type === 'checkbox' ? (
                    <input
                      type="checkbox"
                      checked={Boolean(values[field.id])}
                      onChange={(event) => setValues((current) => ({ ...current, [field.id]: event.target.checked }))}
                    />
                  ) : (
                    <input
                      type={field.type ?? 'text'}
                      value={String(values[field.id] ?? '')}
                      onChange={(event) => setValues((current) => ({ ...current, [field.id]: event.target.value }))}
                      placeholder={field.placeholder}
                      required={field.required}
                      min={field.min}
                      step={field.step}
                    />
                  )}
                </label>
              ))}
            </div>
            <footer className="crud-action-dialog__footer">
              <button type="button" className="btn btn-secondary" onClick={props.onCancel}>
                {props.cancelLabel ?? 'Cancelar'}
              </button>
              <button type="submit" className="btn btn-primary">
                {props.submitLabel ?? 'Guardar'}
              </button>
            </footer>
          </form>
        ) : (
          <>
            <div className="crud-action-dialog__body">
              <pre className="crud-action-dialog__text">{props.textContent}</pre>
            </div>
            <footer className="crud-action-dialog__footer">
              <button type="button" className="btn btn-primary" onClick={props.onCancel}>
                {props.closeLabel ?? 'Cerrar'}
              </button>
            </footer>
          </>
        )}
      </div>
    </div>
  );

  return createPortal(body, document.body);
}
