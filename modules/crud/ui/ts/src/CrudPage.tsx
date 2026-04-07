/**
 * Página CRUD para consolas administrativas.
 *
 * Responsabilidad: orquestar lista, formulario y acciones sobre datos inyectados (`dataSource` o
 * `basePath` + `httpClient`). No contiene reglas de negocio ni llamadas acopladas a un producto.
 *
 * Shell de layout: `core/browser/ts`. Orquestación CRUD: `modules/crud/ui/ts`.
 */
import { FormEvent, type ReactElement, useEffect, useMemo, useRef, useState } from "react";
import { CrudPageShell, parsePaginatedResponse } from "@devpablocristo/core-browser/crud";
import { search as fuzzySearch, type SearchEntry } from "@devpablocristo/core-browser/search";
import { mergeCrudFeatureFlags } from "./crudFeatureFlags";
import { crudItemPath, crudListPath } from "./restPaths";
import { interpolate, mergeCrudStrings, type CrudStrings, defaultCrudStrings } from "./strings";
import type {
  CrudFormValues,
  CrudPageConfig,
  CrudRowAction,
  CrudToolbarAction,
} from "./types";

/**
 * Props extendidas: textos UI genéricos (fusión sobre `stringsBase`).
 */
export type CrudPageProps<T extends { id: string }> = CrudPageConfig<T> & {
  strings?: Partial<CrudStrings>;
  stringsBase?: CrudStrings;
};

function buttonClass(kind: "primary" | "secondary" | "danger" | "success" = "secondary", small = true): string {
  const size = small ? "btn-sm " : "";
  switch (kind) {
    case "primary":
      return `${size}btn-primary`;
    case "danger":
      return `${size}btn-danger`;
    case "success":
      return `${size}btn-success`;
    default:
      return `${size}btn-secondary`;
  }
}

function normalizeError(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

export function CrudPage<T extends { id: string }>(props: CrudPageProps<T>): ReactElement {
  const {
    basePath,
    listQuery,
    dataSource,
    httpClient: httpClientProp,
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
    formatFieldText = (s) => s,
    sentenceCase = (s) => s,
    strings: stringsPartial,
    stringsBase = defaultCrudStrings,
    onExternalEdit,
    preSearchFilter,
    listHeaderInlineSlot,
    listHeaderSlotPlacement = "belowSubtitle",
    externalSearch,
    featureFlags: featureFlagsProp,
    renderTagsCell: _omitRenderTagsCell,
  } = props;

  const featureFlags = useMemo(() => mergeCrudFeatureFlags(featureFlagsProp), [featureFlagsProp]);
  const paginationEnabled = featureFlags.pagination;

  const str = useMemo(() => mergeCrudStrings(stringsBase, stringsPartial), [stringsBase, stringsPartial]);
  const httpClient = httpClientProp;

  const vars = useMemo(
    () => ({
      label,
      labelPlural,
      labelPluralCap,
    }),
    [label, labelPlural, labelPluralCap],
  );

  const [items, setItems] = useState<T[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [internalSearch, setInternalSearch] = useState("");
  const search = externalSearch ?? internalSearch;
  const [showArchived, setShowArchived] = useState(false);

  const [editing, setEditing] = useState<T | null>(null);
  const [creating, setCreating] = useState(false);
  const [formValues, setFormValues] = useState<CrudFormValues>({});
  const [saving, setSaving] = useState(false);

  const [busyKey, setBusyKey] = useState<string | null>(null);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const [confirmDeleteText, setConfirmDeleteText] = useState("");

  // Evita condiciones de carrera (p. ej. React StrictMode doble mount) que dejan loading en true.
  const loadSeqRef = useRef(0);

  const emptyValues = Object.fromEntries(
    formFields.map((field) => [field.key, field.type === "checkbox" ? false : ""]),
  ) as CrudFormValues;
  const activeFormFields = formFields.filter((field) => {
    if (editing && field.createOnly) return false;
    if (!editing && field.editOnly) return false;
    return true;
  });

  const canCreate = allowCreate ?? (formFields.length > 0 && Boolean(dataSource?.create || basePath));
  const canEdit =
    allowEdit ??
    (Boolean(onExternalEdit) || (formFields.length > 0 && Boolean(dataSource?.update || basePath)));
  const canDelete = allowDelete ?? Boolean(dataSource?.deleteItem || basePath);
  const canRestore = allowRestore ?? (supportsArchived && Boolean(dataSource?.restore || basePath));
  const canHardDelete = allowHardDelete ?? (supportsArchived && Boolean(dataSource?.hardDelete || basePath));
  const showForm = (creating || (editing !== null && !onExternalEdit)) && formFields.length > 0;
  const hardDeleteWord = str.confirmWord;

  const defaultPageSize = paginationEnabled ? 100 : 10_000;

  function buildListPath(cursor?: string): string {
    let path = crudListPath(basePath!, showArchived && supportsArchived);
    const params: string[] = [];
    if (listQuery) params.push(listQuery);
    params.push(`limit=${defaultPageSize}`);
    if (cursor) params.push(`after=${cursor}`);
    if (params.length > 0) {
      path = path.includes("?") ? `${path}&${params.join("&")}` : `${path}?${params.join("&")}`;
    }
    return path;
  }

  async function loadItems(): Promise<void> {
    const seq = ++loadSeqRef.current;
    setLoading(true);
    setError("");
    setHasMore(false);
    setNextCursor(null);
    try {
      if (dataSource?.list) {
        const rows = await dataSource.list({ archived: showArchived });
        if (seq !== loadSeqRef.current) return;
        setItems(rows);
        return;
      }
      if (!basePath || !httpClient) {
        if (seq !== loadSeqRef.current) return;
        if (!basePath) {
          setItems([]);
          return;
        }
        setError("CrudPage: basePath requires httpClient or dataSource.list");
        setItems([]);
        return;
      }
      const data = await httpClient.json<unknown>(buildListPath());
      if (seq !== loadSeqRef.current) return;
      const page = parsePaginatedResponse<T>(data);
      setItems(page.items);
      if (paginationEnabled) {
        setHasMore(page.hasMore);
        setNextCursor(page.nextCursor || null);
      } else {
        setHasMore(false);
        setNextCursor(null);
      }
    } catch (err) {
      if (seq === loadSeqRef.current) setError(normalizeError(err));
    } finally {
      if (seq === loadSeqRef.current) setLoading(false);
    }
  }

  async function loadMore(): Promise<void> {
    if (!paginationEnabled || !basePath || !httpClient || !nextCursor) return;
    setLoadingMore(true);
    try {
      const data = await httpClient.json<unknown>(buildListPath(nextCursor));
      const page = parsePaginatedResponse<T>(data);
      setItems((prev) => [...prev, ...page.items]);
      setHasMore(page.hasMore);
      setNextCursor(page.nextCursor || null);
    } catch (err) {
      setError(normalizeError(err));
    } finally {
      setLoadingMore(false);
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
    setConfirmDeleteText("");
  }

  function setField(key: string, value: string | boolean): void {
    setFormValues((current) => ({ ...current, [key]: value }));
  }

  async function submitForm(event: FormEvent): Promise<void> {
    event.preventDefault();
    if (!isValid(formValues)) return;

    setSaving(true);
    setError("");
    try {
      if (editing) {
        if (dataSource?.update) {
          await dataSource.update(editing, formValues);
        } else if (basePath && httpClient) {
          await httpClient.json(crudItemPath(basePath, editing.id), { method: "PUT", body: toBody ? toBody(formValues) : {} });
        }
      } else if (dataSource?.create) {
        await dataSource.create(formValues);
      } else if (basePath && httpClient) {
        await httpClient.json(basePath, { method: "POST", body: toBody ? toBody(formValues) : {} });
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
    setError("");
    try {
      if (dataSource?.deleteItem) {
        await dataSource.deleteItem(row);
      } else if (basePath && httpClient) {
        await httpClient.json(crudItemPath(basePath, row.id), { method: "DELETE" });
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
    setError("");
    try {
      if (dataSource?.restore) {
        await dataSource.restore(row);
      } else if (basePath && httpClient) {
        await httpClient.json(crudItemPath(basePath, row.id, "restore"), { method: "POST", body: {} });
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
    setError("");
    try {
      if (dataSource?.hardDelete) {
        await dataSource.hardDelete(row);
      } else if (basePath && httpClient) {
        await httpClient.json(crudItemPath(basePath, row.id, "hard"), { method: "DELETE" });
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
    setError("");
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
    setError("");
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

  const preSearchItems = useMemo(() => {
    if (!preSearchFilter) return items;
    return preSearchFilter(items);
  }, [items, preSearchFilter]);

  const searchEntries = useMemo<SearchEntry<T>[]>(
    () => preSearchItems.map((row) => ({ item: row, text: searchText(row) })),
    [preSearchItems, searchText],
  );

  const filtered = useMemo(() => {
    const q = search.trim();
    if (q.length === 0) return preSearchItems;
    return fuzzySearch(q, searchEntries).map((r) => r.item);
  }, [search, preSearchItems, searchEntries]);

  const visibleToolbarActions = toolbarActions.filter((action) => action.isVisible?.({ archived: showArchived, items }) ?? true);
  const showToolbarButtonRow =
    visibleToolbarActions.length > 0 || canCreate || supportsArchived;

  const searchPlaceholderResolved =
    searchPlaceholder != null && searchPlaceholder !== ""
      ? formatFieldText(searchPlaceholder)
      : interpolate(str.searchPlaceholder, vars);

  const titleActive = sentenceCase(labelPluralCap);
  const titleArchivedView = sentenceCase(interpolate(str.titleArchived, { ...vars, labelPluralCap }));

  return (
    <CrudPageShell
      title={showArchived ? titleArchivedView : titleActive}
      subtitle={
        loading
          ? str.statusLoading
          : `${filtered.length} ${filtered.length === 1 ? label : labelPlural}`
      }
      headerBeforeTitleSlot={
        listHeaderSlotPlacement === "aboveTitle" && listHeaderInlineSlot != null ? (
          <div className="crud-list-header-lead crud-list-header-lead--above-title">
            {listHeaderInlineSlot({ items })}
          </div>
        ) : undefined
      }
      headerLeadSlot={
        listHeaderSlotPlacement !== "aboveTitle" && listHeaderInlineSlot != null ? (
          <div className="crud-list-header-lead">{listHeaderInlineSlot({ items })}</div>
        ) : undefined
      }
      search={externalSearch == null ? {
        value: internalSearch,
        onChange: setInternalSearch,
        placeholder: searchPlaceholderResolved,
        inputClassName: "m-kanban__search",
      } : undefined}
      headerActions={showToolbarButtonRow ? (
        <>
          {visibleToolbarActions.map((action) => (
            <button
              key={action.id}
              type="button"
              className={buttonClass(action.kind)}
              onClick={() => {
                void runToolbarAction(action);
              }}
            >
              {formatFieldText(action.label)}
            </button>
          ))}
          {canCreate && (
            <button type="button" className="btn-sm btn-primary" onClick={openCreate}>
              {createLabel
                ? formatFieldText(createLabel)
                : sentenceCase(interpolate(str.buttonNew, vars))}
            </button>
          )}
          {supportsArchived && (
            <button
              type="button"
              className={`btn-sm ${showArchived ? "btn-primary" : "btn-secondary"}`}
              onClick={() => {
                closeForm();
                cancelHardDelete();
                setShowArchived((current) => !current);
              }}
            >
              {showArchived ? str.toggleShowActive : str.toggleShowArchived}
            </button>
          )}
        </>
      ) : undefined}
      error={error ? <div className="alert alert-error">{error}</div> : undefined}
      form={
        showForm && (!showArchived || creating) ? (
          <div className="card crud-form-card">
            <div className="card-header">
              <h2>
                {sentenceCase(
                  interpolate(editing ? str.formEdit : str.formCreate, vars),
                )}
              </h2>
            </div>
            <form
              onSubmit={(event) => {
                void submitForm(event);
              }}
              className="crud-form"
            >
              <div className="crud-form-grid">
                {activeFormFields.map((field) => (
                  <div key={field.key} className={`form-group${field.fullWidth ? " full-width" : ""}`}>
                    <label htmlFor={`crud-field-${field.key}`}>
                      {formatFieldText(field.label)}
                      {field.required ? " *" : ""}
                    </label>
                    {field.type === "textarea" ? (
                      <textarea
                        id={`crud-field-${field.key}`}
                        rows={3}
                        value={String(formValues[field.key] ?? "")}
                        onChange={(event) => setField(field.key, event.target.value)}
                        placeholder={field.placeholder ? formatFieldText(field.placeholder) : undefined}
                      />
                    ) : field.type === "select" ? (
                      <select
                        id={`crud-field-${field.key}`}
                        value={String(formValues[field.key] ?? "")}
                        onChange={(event) => setField(field.key, event.target.value)}
                      >
                        <option value="">{field.placeholder ? formatFieldText(field.placeholder) : str.selectPlaceholder}</option>
                        {(field.options ?? []).map((option) => (
                          <option key={option.value} value={option.value}>
                            {formatFieldText(option.label)}
                          </option>
                        ))}
                      </select>
                    ) : field.type === "checkbox" ? (
                      <label className="toggle">
                        <input
                          id={`crud-field-${field.key}`}
                          aria-label={formatFieldText(field.label)}
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
                        type={field.type ?? "text"}
                        value={String(formValues[field.key] ?? "")}
                        onChange={(event) => setField(field.key, event.target.value)}
                        placeholder={field.placeholder ? formatFieldText(field.placeholder) : undefined}
                        autoFocus={field === activeFormFields[0]}
                      />
                    )}
                  </div>
                ))}
              </div>
              <div className="actions-row">
                <button type="submit" className="btn-primary" disabled={saving || !isValid(formValues)}>
                  {saving ? str.statusSaving : str.actionSave}
                </button>
                <button type="button" className="btn-secondary" onClick={closeForm} disabled={saving}>
                  {str.actionCancel}
                </button>
              </div>
            </form>
          </div>
        ) : undefined
      }
    >
      {loading ? (
        <div className="spinner" />
      ) : filtered.length === 0 ? (
        <div className="empty-state">
          <p>
            {search.trim()
              ? interpolate(str.emptySearch, { ...vars, search: search.trim() })
              : showArchived
                ? archivedEmptyState
                  ? formatFieldText(archivedEmptyState)
                  : interpolate(str.emptyArchived, vars)
                : emptyState
                  ? formatFieldText(emptyState)
                  : interpolate(str.emptyActive, vars)}
          </p>
          {!search.trim() && canCreate && (
            <button type="button" className="btn-primary" onClick={openCreate}>
              {createLabel
                ? formatFieldText(createLabel)
                : sentenceCase(interpolate(str.buttonCreateFirst, vars))}
            </button>
          )}
        </div>
      ) : (
        <div className="table-wrap">
          <table className="crud-table">
            <thead>
              <tr>
                {columns.map((column) => (
                  <th key={column.key} className={column.className}>
                    {sentenceCase(formatFieldText(column.header))}
                  </th>
                ))}
                <th className="col-actions">{sentenceCase(str.tableActions)}</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((row) => {
                const visibleRowActions = rowActions.filter(
                  (action) => action.isVisible?.(row, { archived: showArchived }) ?? true,
                );
                return (
                  <tr key={row.id}>
                    {columns.map((column) => (
                      <td key={column.key} className={column.className}>
                        {column.render ? column.render(row[column.key], row) : String(row[column.key] ?? "") || "---"}
                      </td>
                    ))}
                    <td className="col-actions">
                      {showArchived ? (
                        <div className="crud-row-actions">
                          {canRestore && (
                            <button
                              type="button"
                              className="btn-sm btn-primary"
                              disabled={busyKey === `${row.id}:restore`}
                              onClick={() => {
                                void restoreRow(row);
                              }}
                            >
                              {busyKey === `${row.id}:restore` ? "..." : str.actionRestore}
                            </button>
                          )}
                          {canHardDelete &&
                            (confirmDeleteId === row.id ? (
                              <div className="confirm-delete-inline" role="group" aria-label={`${str.actionDelete} ${label}`}>
                                <div className="confirm-delete-copy">
                                  <span className="confirm-delete-hint">
                                    {interpolate(str.confirmHint, { word: hardDeleteWord })}
                                  </span>
                                </div>
                                <input
                                  type="text"
                                  className="confirm-delete-input"
                                  value={confirmDeleteText}
                                  onChange={(event) => setConfirmDeleteText(event.target.value)}
                                  placeholder={str.confirmPlaceholder}
                                  autoFocus
                                />
                                <div className="confirm-delete-actions">
                                  <button
                                    type="button"
                                    className="btn-sm btn-danger"
                                    disabled={
                                      confirmDeleteText.toLowerCase() !== hardDeleteWord.toLowerCase() ||
                                      busyKey === `${row.id}:hard-delete`
                                    }
                                    onClick={() => {
                                      void hardDeleteRow(row);
                                    }}
                                  >
                                    {busyKey === `${row.id}:hard-delete` ? "..." : str.actionConfirm}
                                  </button>
                                  <button type="button" className="btn-sm btn-secondary" onClick={cancelHardDelete}>
                                    {str.actionCancel}
                                  </button>
                                </div>
                              </div>
                            ) : (
                              <button
                                type="button"
                                className="btn-sm btn-danger"
                                disabled={busyKey === `${row.id}:hard-delete`}
                                onClick={() => {
                                  setConfirmDeleteId(row.id);
                                  setConfirmDeleteText("");
                                }}
                              >
                                {str.actionDelete}
                              </button>
                            ))}
                        </div>
                      ) : (
                        <div className="crud-row-actions">
                          {canEdit && (
                            <button
                              type="button"
                              className="btn-sm btn-secondary"
                              onClick={() => (onExternalEdit ? onExternalEdit(row) : openEdit(row))}
                            >
                              {str.actionEdit}
                            </button>
                          )}
                          {visibleRowActions.map((action) => (
                            <button
                              key={action.id}
                              type="button"
                              className={buttonClass(action.kind)}
                              disabled={busyKey === `${row.id}:${action.id}`}
                              onClick={() => {
                                void runRowAction(action, row);
                              }}
                            >
                              {busyKey === `${row.id}:${action.id}` ? "..." : formatFieldText(action.label)}
                            </button>
                          ))}
                          {canDelete && (
                            <button
                              type="button"
                              className="btn-sm btn-danger"
                              disabled={busyKey === `${row.id}:delete`}
                              onClick={() => {
                                void deleteRow(row);
                              }}
                            >
                              {busyKey === `${row.id}:delete` ? "..." : supportsArchived ? str.actionArchive : str.actionDelete}
                            </button>
                          )}
                        </div>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
          {paginationEnabled && hasMore && (
            <div className="crud-load-more">
              <button
                type="button"
                className="btn-secondary"
                disabled={loadingMore}
                onClick={() => { void loadMore(); }}
              >
                {loadingMore ? str.statusLoading : str.loadMore}
              </button>
            </div>
          )}
        </div>
      )}
    </CrudPageShell>
  );
}
