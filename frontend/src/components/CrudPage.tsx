import { FormEvent, type ReactNode, useEffect, useState } from 'react';
import { apiRequest } from '../lib/api';

export type CrudFieldValue = string | boolean;
export type CrudFormValues = Record<string, CrudFieldValue>;

export type CrudColumn<T> = {
  key: keyof T & string;
  header: string;
  render?: (value: unknown, row: T) => ReactNode;
  className?: string;
};

export type CrudFormField = {
  key: string;
  label: string;
  type?: 'text' | 'email' | 'tel' | 'number' | 'date' | 'datetime-local' | 'textarea' | 'select' | 'checkbox';
  placeholder?: string;
  required?: boolean;
  fullWidth?: boolean;
  createOnly?: boolean;
  editOnly?: boolean;
  options?: Array<{ label: string; value: string }>;
};

export type CrudDataSource<T extends { id: string }> = {
  list?: (params: { archived: boolean }) => Promise<T[]>;
  create?: (values: CrudFormValues) => Promise<unknown>;
  update?: (row: T, values: CrudFormValues) => Promise<unknown>;
  deleteItem?: (row: T) => Promise<unknown>;
  restore?: (row: T) => Promise<unknown>;
  hardDelete?: (row: T) => Promise<unknown>;
};

type CrudHelpers<T extends { id: string }> = {
  items: T[];
  reload: () => Promise<void>;
  setError: (message: string) => void;
};

export type CrudToolbarAction<T extends { id: string }> = {
  id: string;
  label: string;
  kind?: 'primary' | 'secondary' | 'danger' | 'success';
  isVisible?: (ctx: { archived: boolean; items: T[] }) => boolean;
  onClick: (helpers: CrudHelpers<T>) => Promise<void> | void;
};

export type CrudRowAction<T extends { id: string }> = {
  id: string;
  label: string;
  kind?: 'primary' | 'secondary' | 'danger' | 'success';
  isVisible?: (row: T, ctx: { archived: boolean }) => boolean;
  onClick: (row: T, helpers: CrudHelpers<T>) => Promise<void> | void;
};

export type CrudPageConfig<T extends { id: string }> = {
  basePath?: string;
  dataSource?: CrudDataSource<T>;
  supportsArchived?: boolean;
  allowCreate?: boolean;
  allowEdit?: boolean;
  allowDelete?: boolean;
  allowRestore?: boolean;
  allowHardDelete?: boolean;
  label: string;
  labelPlural: string;
  labelPluralCap: string;
  columns: CrudColumn<T>[];
  formFields: CrudFormField[];
  searchText: (row: T) => string;
  toFormValues: (row: T) => CrudFormValues;
  toBody?: (values: CrudFormValues) => Record<string, unknown>;
  isValid: (values: CrudFormValues) => boolean;
  searchPlaceholder?: string;
  emptyState?: string;
  archivedEmptyState?: string;
  createLabel?: string;
  toolbarActions?: CrudToolbarAction<T>[];
  rowActions?: CrudRowAction<T>[];
};

function parseListResponse<T>(data: { items?: T[] } | T[]): T[] {
  return Array.isArray(data) ? data : (data.items ?? []);
}

function buttonClass(kind: 'primary' | 'secondary' | 'danger' | 'success' = 'secondary', small = true): string {
  const size = small ? 'btn-sm ' : '';
  switch (kind) {
    case 'primary':
      return `${size}btn-primary`;
    case 'danger':
      return `${size}btn-danger`;
    case 'success':
      return `${size}btn-success`;
    default:
      return `${size}btn-secondary`;
  }
}

function normalizeError(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

export function CrudPage<T extends { id: string }>({
  basePath,
  dataSource,
  supportsArchived = false,
  allowCreate,
  allowEdit,
  allowDelete,
  allowRestore,
  allowHardDelete,
  label,
  labelPlural,
  labelPluralCap,
  columns,
  formFields,
  searchText,
  toFormValues,
  toBody,
  isValid,
  searchPlaceholder,
  emptyState,
  archivedEmptyState,
  createLabel,
  toolbarActions = [],
  rowActions = [],
}: CrudPageConfig<T>) {
  const [items, setItems] = useState<T[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [showArchived, setShowArchived] = useState(false);

  const [editing, setEditing] = useState<T | null>(null);
  const [creating, setCreating] = useState(false);
  const [formValues, setFormValues] = useState<CrudFormValues>({});
  const [saving, setSaving] = useState(false);

  const [busyKey, setBusyKey] = useState<string | null>(null);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const [confirmDeleteText, setConfirmDeleteText] = useState('');

  const emptyValues = Object.fromEntries(
    formFields.map((field) => [field.key, field.type === 'checkbox' ? false : '']),
  ) as CrudFormValues;
  const activeFormFields = formFields.filter((field) => {
    if (editing && field.createOnly) return false;
    if (!editing && field.editOnly) return false;
    return true;
  });

  const canCreate = allowCreate ?? (formFields.length > 0 && Boolean(dataSource?.create || basePath));
  const canEdit = allowEdit ?? (formFields.length > 0 && Boolean(dataSource?.update || basePath));
  const canDelete = allowDelete ?? Boolean(dataSource?.deleteItem || basePath);
  const canRestore = allowRestore ?? (supportsArchived && Boolean(dataSource?.restore || basePath));
  const canHardDelete = allowHardDelete ?? (supportsArchived && Boolean(dataSource?.hardDelete || basePath));
  const showForm = (creating || editing !== null) && formFields.length > 0;

  async function loadItems(): Promise<void> {
    setLoading(true);
    setError('');
    try {
      if (dataSource?.list) {
        setItems(await dataSource.list({ archived: showArchived }));
        return;
      }
      if (!basePath) {
        setItems([]);
        return;
      }
      const path = showArchived && supportsArchived ? `${basePath}/archived` : basePath;
      const data = await apiRequest<{ items?: T[] } | T[]>(path);
      setItems(parseListResponse(data));
    } catch (err) {
      setError(normalizeError(err));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadItems();
  }, [showArchived]);

  function closeForm(): void {
    setCreating(false);
    setEditing(null);
    setFormValues({});
  }

  function openCreate(): void {
    setEditing(null);
    setCreating(true);
    setFormValues({ ...emptyValues });
  }

  function openEdit(row: T): void {
    setCreating(false);
    setEditing(row);
    setFormValues(toFormValues(row));
  }

  function cancelHardDelete(): void {
    setConfirmDeleteId(null);
    setConfirmDeleteText('');
  }

  function setField(key: string, value: CrudFieldValue): void {
    setFormValues((current) => ({ ...current, [key]: value }));
  }

  async function submitForm(event: FormEvent): Promise<void> {
    event.preventDefault();
    if (!isValid(formValues)) return;

    setSaving(true);
    setError('');
    try {
      if (editing) {
        if (dataSource?.update) {
          await dataSource.update(editing, formValues);
        } else if (basePath) {
          await apiRequest(`${basePath}/${editing.id}`, { method: 'PUT', body: toBody ? toBody(formValues) : {} });
        }
      } else if (dataSource?.create) {
        await dataSource.create(formValues);
      } else if (basePath) {
        await apiRequest(basePath, { method: 'POST', body: toBody ? toBody(formValues) : {} });
      }
      closeForm();
      await loadItems();
    } catch (err) {
      setError(normalizeError(err));
    } finally {
      setSaving(false);
    }
  }

  async function deleteRow(row: T): Promise<void> {
    const nextBusyKey = `${row.id}:delete`;
    setBusyKey(nextBusyKey);
    setError('');
    try {
      if (dataSource?.deleteItem) {
        await dataSource.deleteItem(row);
      } else if (basePath) {
        await apiRequest(`${basePath}/${row.id}`, { method: 'DELETE' });
      }
      await loadItems();
    } catch (err) {
      setError(normalizeError(err));
    } finally {
      setBusyKey(null);
    }
  }

  async function restoreRow(row: T): Promise<void> {
    const nextBusyKey = `${row.id}:restore`;
    setBusyKey(nextBusyKey);
    setError('');
    try {
      if (dataSource?.restore) {
        await dataSource.restore(row);
      } else if (basePath) {
        await apiRequest(`${basePath}/${row.id}/restore`, { method: 'POST', body: {} });
      }
      await loadItems();
    } catch (err) {
      setError(normalizeError(err));
    } finally {
      setBusyKey(null);
    }
  }

  async function hardDeleteRow(row: T): Promise<void> {
    const nextBusyKey = `${row.id}:hard-delete`;
    setBusyKey(nextBusyKey);
    setError('');
    try {
      if (dataSource?.hardDelete) {
        await dataSource.hardDelete(row);
      } else if (basePath) {
        await apiRequest(`${basePath}/${row.id}/hard`, { method: 'DELETE' });
      }
      cancelHardDelete();
      await loadItems();
    } catch (err) {
      setError(normalizeError(err));
    } finally {
      setBusyKey(null);
    }
  }

  async function runToolbarAction(action: CrudToolbarAction<T>): Promise<void> {
    setError('');
    try {
      await action.onClick({
        items,
        reload: loadItems,
        setError,
      });
    } catch (err) {
      setError(normalizeError(err));
    }
  }

  async function runRowAction(action: CrudRowAction<T>, row: T): Promise<void> {
    const nextBusyKey = `${row.id}:${action.id}`;
    setBusyKey(nextBusyKey);
    setError('');
    try {
      await action.onClick(row, {
        items,
        reload: loadItems,
        setError,
      });
    } catch (err) {
      setError(normalizeError(err));
    } finally {
      setBusyKey(null);
    }
  }

  const filtered = items.filter((row) => {
    if (!search.trim()) return true;
    return searchText(row).toLowerCase().includes(search.trim().toLowerCase());
  });

  const visibleToolbarActions = toolbarActions.filter((action) => action.isVisible?.({ archived: showArchived, items }) ?? true);

  return (
    <>
      <div className="page-header">
        <div>
          <h1>{showArchived ? `${labelPluralCap} archivados` : labelPluralCap}</h1>
          <p className="text-secondary">
            {loading ? 'Cargando...' : `${filtered.length} ${filtered.length === 1 ? label : labelPlural}`}
          </p>
        </div>
        <div className="actions-row">
          {visibleToolbarActions.map((action) => (
            <button
              key={action.id}
              type="button"
              className={buttonClass(action.kind, false)}
              onClick={() => { void runToolbarAction(action); }}
            >
              {action.label}
            </button>
          ))}
          {!showArchived && canCreate && (
            <button type="button" className="btn-primary" onClick={openCreate}>
              {createLabel ?? `+ Nuevo ${label}`}
            </button>
          )}
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {showForm && !showArchived && (
        <div className="card crud-form-card">
          <div className="card-header">
            <h2>{editing ? `Editar ${label}` : `Nuevo ${label}`}</h2>
          </div>
          <form onSubmit={(event) => { void submitForm(event); }} className="crud-form">
            <div className="crud-form-grid">
              {activeFormFields.map((field) => (
                <div key={field.key} className={`form-group${field.fullWidth ? ' full-width' : ''}`}>
                  <label htmlFor={`crud-field-${field.key}`}>{field.label}{field.required ? ' *' : ''}</label>
                  {field.type === 'textarea' ? (
                    <textarea
                      id={`crud-field-${field.key}`}
                      rows={3}
                      value={String(formValues[field.key] ?? '')}
                      onChange={(event) => setField(field.key, event.target.value)}
                      placeholder={field.placeholder}
                    />
                  ) : field.type === 'select' ? (
                    <select
                      id={`crud-field-${field.key}`}
                      value={String(formValues[field.key] ?? '')}
                      onChange={(event) => setField(field.key, event.target.value)}
                    >
                      <option value="">{field.placeholder ?? 'Seleccionar...'}</option>
                      {(field.options ?? []).map((option) => (
                        <option key={option.value} value={option.value}>
                          {option.label}
                        </option>
                      ))}
                    </select>
                  ) : field.type === 'checkbox' ? (
                    <label className="toggle">
                      <input
                        id={`crud-field-${field.key}`}
                        aria-label={field.label}
                        type="checkbox"
                        checked={Boolean(formValues[field.key])}
                        onChange={(event) => setField(field.key, event.target.checked)}
                      />
                      <span className="toggle-track" />
                      <span className="toggle-thumb" />
                    </label>
                  ) : (
                    <input
                      id={`crud-field-${field.key}`}
                      type={field.type ?? 'text'}
                      value={String(formValues[field.key] ?? '')}
                      onChange={(event) => setField(field.key, event.target.value)}
                      placeholder={field.placeholder}
                      autoFocus={field === activeFormFields[0]}
                    />
                  )}
                </div>
              ))}
            </div>
            <div className="actions-row">
              <button type="submit" className="btn-primary" disabled={saving || !isValid(formValues)}>
                {saving ? 'Guardando...' : 'Guardar'}
              </button>
              <button type="button" className="btn-secondary" onClick={closeForm} disabled={saving}>
                Cancelar
              </button>
            </div>
          </form>
        </div>
      )}

      <div className="crud-toolbar">
        <input
          type="text"
          className="crud-search"
          placeholder={searchPlaceholder ?? `Buscar ${labelPlural}...`}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        {supportsArchived && (
          <button
            type="button"
            className={`btn-sm ${showArchived ? 'btn-primary' : 'btn-secondary'}`}
            onClick={() => {
              closeForm();
              cancelHardDelete();
              setShowArchived((current) => !current);
            }}
          >
            {showArchived ? 'Ver activos' : 'Ver archivados'}
          </button>
        )}
      </div>

      {loading ? (
        <div className="spinner" />
      ) : filtered.length === 0 ? (
        <div className="empty-state">
          <p>
            {search.trim()
              ? `No se encontraron ${labelPlural} con "${search.trim()}"`
              : showArchived
                ? (archivedEmptyState ?? `No hay ${labelPlural} archivados.`)
                : (emptyState ?? `No hay ${labelPlural} registrados.`)}
          </p>
          {!search.trim() && !showArchived && canCreate && (
            <button type="button" className="btn-primary" onClick={openCreate}>
              {createLabel ?? `+ Crear primer ${label}`}
            </button>
          )}
        </div>
      ) : (
        <div className="table-wrap">
          <table className="crud-table">
            <thead>
              <tr>
                {columns.map((column) => (
                  <th key={column.key} className={column.className}>{column.header}</th>
                ))}
                <th className="col-actions">Acciones</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((row) => {
                const visibleRowActions = rowActions.filter((action) => action.isVisible?.(row, { archived: showArchived }) ?? true);
                return (
                  <tr key={row.id}>
                    {columns.map((column) => (
                      <td key={column.key} className={column.className}>
                        {column.render ? column.render(row[column.key], row) : (String(row[column.key] ?? '') || '---')}
                      </td>
                    ))}
                    <td className="col-actions">
                      {showArchived ? (
                        <>
                          {canRestore && (
                            <button
                              type="button"
                              className="btn-sm btn-primary"
                              disabled={busyKey === `${row.id}:restore`}
                              onClick={() => { void restoreRow(row); }}
                            >
                              {busyKey === `${row.id}:restore` ? '...' : 'Restaurar'}
                            </button>
                          )}
                          {canHardDelete && (
                            confirmDeleteId === row.id ? (
                              <div className="confirm-delete-inline">
                                <span className="confirm-delete-hint">Escribi <strong>eliminar</strong> para confirmar</span>
                                <input
                                  type="text"
                                  className="confirm-delete-input"
                                  value={confirmDeleteText}
                                  onChange={(event) => setConfirmDeleteText(event.target.value)}
                                  placeholder="eliminar"
                                  autoFocus
                                />
                                <button
                                  type="button"
                                  className="btn-sm btn-danger"
                                  disabled={confirmDeleteText.toLowerCase() !== 'eliminar' || busyKey === `${row.id}:hard-delete`}
                                  onClick={() => { void hardDeleteRow(row); }}
                                >
                                  {busyKey === `${row.id}:hard-delete` ? '...' : 'Confirmar'}
                                </button>
                                <button type="button" className="btn-sm btn-secondary" onClick={cancelHardDelete}>
                                  Cancelar
                                </button>
                              </div>
                            ) : (
                              <button
                                type="button"
                                className="btn-sm btn-danger"
                                disabled={busyKey === `${row.id}:hard-delete`}
                                onClick={() => {
                                  setConfirmDeleteId(row.id);
                                  setConfirmDeleteText('');
                                }}
                              >
                                Eliminar
                              </button>
                            )
                          )}
                        </>
                      ) : (
                        <>
                          {canEdit && (
                            <button type="button" className="btn-sm btn-secondary" onClick={() => openEdit(row)}>
                              Editar
                            </button>
                          )}
                          {visibleRowActions.map((action) => (
                            <button
                              key={action.id}
                              type="button"
                              className={buttonClass(action.kind)}
                              disabled={busyKey === `${row.id}:${action.id}`}
                              onClick={() => { void runRowAction(action, row); }}
                            >
                              {busyKey === `${row.id}:${action.id}` ? '...' : action.label}
                            </button>
                          ))}
                          {canDelete && (
                            <button
                              type="button"
                              className="btn-sm btn-danger"
                              disabled={busyKey === `${row.id}:delete`}
                              onClick={() => { void deleteRow(row); }}
                            >
                              {busyKey === `${row.id}:delete` ? '...' : supportsArchived ? 'Archivar' : 'Eliminar'}
                            </button>
                          )}
                        </>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
