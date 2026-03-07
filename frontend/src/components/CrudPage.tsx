import { FormEvent, type ReactNode, useEffect, useState } from 'react';
import { apiRequest } from '../lib/api';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type CrudColumn<T> = {
  key: keyof T & string;
  header: string;
  render?: (value: unknown, row: T) => ReactNode;
  className?: string;
};

export type CrudFormField = {
  key: string;
  label: string;
  type?: 'text' | 'email' | 'tel' | 'number' | 'date' | 'textarea';
  placeholder?: string;
  required?: boolean;
  fullWidth?: boolean;
};

export type CrudPageConfig<T extends { id: string }> = {
  /** API base path, e.g. "/v1/customers" */
  basePath: string;
  /** Singular label, e.g. "alumno" */
  label: string;
  /** Plural label, e.g. "alumnos" */
  labelPlural: string;
  /** Capitalized plural, e.g. "Alumnos" */
  labelPluralCap: string;
  /** Table columns to render */
  columns: CrudColumn<T>[];
  /** Form field definitions */
  formFields: CrudFormField[];
  /** Extract search text from a row for filtering */
  searchText: (row: T) => string;
  /** Convert a row into form values for editing */
  toFormValues: (row: T) => Record<string, string>;
  /** Convert form values into API body for create/update */
  toBody: (values: Record<string, string>) => Record<string, unknown>;
  /** Validate form — return true if valid */
  isValid: (values: Record<string, string>) => boolean;
};

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function CrudPage<T extends { id: string }>({
  basePath,
  label,
  labelPlural,
  labelPluralCap,
  columns,
  formFields,
  searchText,
  toFormValues,
  toBody,
  isValid,
}: CrudPageConfig<T>) {
  const [items, setItems] = useState<T[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [showArchived, setShowArchived] = useState(false);

  // Form state
  const [editing, setEditing] = useState<T | null>(null);
  const [creating, setCreating] = useState(false);
  const [formValues, setFormValues] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  // Action state
  const [busy, setBusy] = useState<string | null>(null);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const [confirmDeleteText, setConfirmDeleteText] = useState('');

  // ---- Data loading ----

  async function loadActive() {
    setLoading(true);
    setError('');
    try {
      const data = await apiRequest<{ items?: T[] } | T[]>(basePath);
      const list = Array.isArray(data) ? data : (data.items ?? []);
      setItems(list);
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  }

  async function loadArchived() {
    setLoading(true);
    setError('');
    try {
      const data = await apiRequest<{ items?: T[] } | T[]>(`${basePath}/archived`);
      const list = Array.isArray(data) ? data : (data.items ?? []);
      setItems(list);
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  }

  function reload() {
    if (showArchived) {
      void loadArchived();
    } else {
      void loadActive();
    }
  }

  useEffect(() => { reload(); }, [showArchived]);

  // ---- Filtering ----

  const filtered = items.filter((row) => {
    if (!search) return true;
    return searchText(row).toLowerCase().includes(search.toLowerCase());
  });

  // ---- Form ----

  const emptyValues = Object.fromEntries(formFields.map((f) => [f.key, '']));

  function openCreate() {
    setEditing(null);
    setFormValues({ ...emptyValues });
    setCreating(true);
  }

  function openEdit(row: T) {
    setCreating(false);
    setEditing(row);
    setFormValues(toFormValues(row));
  }

  function closeForm() {
    setCreating(false);
    setEditing(null);
    setFormValues({});
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!isValid(formValues)) return;
    setSaving(true);
    setError('');
    try {
      if (editing) {
        await apiRequest(`${basePath}/${editing.id}`, { method: 'PUT', body: toBody(formValues) });
      } else {
        await apiRequest(basePath, { method: 'POST', body: toBody(formValues) });
      }
      closeForm();
      reload();
    } catch (err) {
      setError(String(err));
    } finally {
      setSaving(false);
    }
  }

  function setField(key: string, value: string) {
    setFormValues((prev) => ({ ...prev, [key]: value }));
  }

  // ---- Archive / Restore / Hard Delete ----

  async function handleArchive(id: string) {
    setBusy(id);
    setError('');
    try {
      await apiRequest(`${basePath}/${id}`, { method: 'DELETE' });
      reload();
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(null);
    }
  }

  async function handleRestore(id: string) {
    setBusy(id);
    setError('');
    try {
      await apiRequest(`${basePath}/${id}/restore`, { method: 'POST', body: {} });
      reload();
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(null);
    }
  }

  function startHardDelete(id: string) {
    setConfirmDeleteId(id);
    setConfirmDeleteText('');
  }

  function cancelHardDelete() {
    setConfirmDeleteId(null);
    setConfirmDeleteText('');
  }

  async function executeHardDelete(id: string) {
    setBusy(id);
    setError('');
    try {
      await apiRequest(`${basePath}/${id}/hard`, { method: 'DELETE' });
      cancelHardDelete();
      reload();
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(null);
    }
  }

  // ---- Render ----

  const showForm = creating || editing !== null;

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
          {!showArchived && (
            <button type="button" className="btn-primary" onClick={openCreate}>
              + Nuevo {label}
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
          <form onSubmit={handleSubmit} className="crud-form">
            <div className="crud-form-grid">
              {formFields.map((field) => (
                <div key={field.key} className={`form-group${field.fullWidth ? ' full-width' : ''}`}>
                  <label>{field.label}{field.required ? ' *' : ''}</label>
                  {field.type === 'textarea' ? (
                    <textarea
                      rows={2}
                      value={formValues[field.key] ?? ''}
                      onChange={(e) => setField(field.key, e.target.value)}
                      placeholder={field.placeholder}
                    />
                  ) : (
                    <input
                      type={field.type ?? 'text'}
                      value={formValues[field.key] ?? ''}
                      onChange={(e) => setField(field.key, e.target.value)}
                      placeholder={field.placeholder}
                      autoFocus={field === formFields[0]}
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
          placeholder={`Buscar ${labelPlural}...`}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <button
          type="button"
          className={`btn-sm ${showArchived ? 'btn-primary' : 'btn-secondary'}`}
          onClick={() => { closeForm(); cancelHardDelete(); setShowArchived((v) => !v); }}
        >
          {showArchived ? 'Ver activos' : 'Ver archivados'}
        </button>
      </div>

      {loading ? (
        <div className="spinner" />
      ) : filtered.length === 0 ? (
        <div className="empty-state">
          <p>
            {search
              ? `No se encontraron ${labelPlural} con "${search}"`
              : showArchived
                ? `No hay ${labelPlural} archivados.`
                : `No hay ${labelPlural} registrados.`}
          </p>
          {!search && !showArchived && (
            <button type="button" className="btn-primary" onClick={openCreate}>
              + Crear primer {label}
            </button>
          )}
        </div>
      ) : (
        <div className="table-wrap">
          <table className="crud-table">
            <thead>
              <tr>
                {columns.map((col) => (
                  <th key={col.key} className={col.className}>{col.header}</th>
                ))}
                <th className="col-actions">Acciones</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((row) => (
                <tr key={row.id}>
                  {columns.map((col) => (
                    <td key={col.key} className={col.className}>
                      {col.render
                        ? col.render(row[col.key], row)
                        : (String(row[col.key] ?? '') || '---')}
                    </td>
                  ))}
                  <td className="col-actions">
                    {showArchived ? (
                      <>
                        {confirmDeleteId === row.id ? (
                          <div className="confirm-delete-inline">
                            <span className="confirm-delete-hint">Escribi <strong>eliminar</strong> para confirmar</span>
                            <input
                              type="text"
                              className="confirm-delete-input"
                              value={confirmDeleteText}
                              onChange={(e) => setConfirmDeleteText(e.target.value)}
                              placeholder="eliminar"
                              autoFocus
                            />
                            <button
                              type="button"
                              className="btn-sm btn-danger"
                              disabled={confirmDeleteText.toLowerCase() !== 'eliminar' || busy === row.id}
                              onClick={() => executeHardDelete(row.id)}
                            >
                              {busy === row.id ? '...' : 'Confirmar'}
                            </button>
                            <button type="button" className="btn-sm btn-secondary" onClick={cancelHardDelete}>
                              Cancelar
                            </button>
                          </div>
                        ) : (
                          <>
                            <button
                              type="button"
                              className="btn-sm btn-primary"
                              disabled={busy === row.id}
                              onClick={() => handleRestore(row.id)}
                            >
                              {busy === row.id ? '...' : 'Restaurar'}
                            </button>
                            <button
                              type="button"
                              className="btn-sm btn-danger"
                              disabled={busy === row.id}
                              onClick={() => startHardDelete(row.id)}
                            >
                              Eliminar
                            </button>
                          </>
                        )}
                      </>
                    ) : (
                      <>
                        <button type="button" className="btn-sm btn-secondary" onClick={() => openEdit(row)}>
                          Editar
                        </button>
                        <button
                          type="button"
                          className="btn-sm btn-danger"
                          disabled={busy === row.id}
                          onClick={() => handleArchive(row.id)}
                        >
                          {busy === row.id ? '...' : 'Archivar'}
                        </button>
                      </>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
