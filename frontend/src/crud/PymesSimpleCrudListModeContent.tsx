import { parsePaginatedResponse } from '@devpablocristo/core-browser/crud';
import { crudItemPath, type CrudFieldValue, type CrudHelpers, type CrudRowAction } from '@devpablocristo/modules-crud-ui';
import { useCallback, useMemo, useState } from 'react';
import './PymesSimpleCrudListModeContent.css';
import { apiRequest } from '../lib/api';
import { useI18n } from '../lib/i18n';
import { PymesCrudResourceShellHeader } from './PymesCrudResourceShellHeader';
import { usePymesCrudConfigQuery } from './usePymesCrudConfigQuery';
import { usePymesCrudHeaderFeatures } from './usePymesCrudHeaderFeatures';
import {
  CrudGallerySurface,
  CrudTableSurface,
  CrudValueKanbanSurface,
  openCrudFormDialog,
  useCrudArchivedSearchParam,
  useCrudRemoteGalleryPage,
  type CrudActionDialogField,
  type CrudTableSurfaceColumn,
  type CrudTableSurfaceRowAction,
} from '../modules/crud';
import type {
  CrudColumn,
  CrudFormField,
  CrudFormValues,
  CrudPageConfig,
  CrudValueFilterOption,
  CrudViewModeId,
} from '../components/CrudPage';

type CrudListResponse<T> = {
  items: T[];
  has_more?: boolean;
  next_cursor?: string | null;
};

function emptyValueForField(field: CrudFormField): CrudFieldValue {
  return field.type === 'checkbox' ? false : '';
}

function toDialogField(field: CrudFormField, values: CrudFormValues): CrudActionDialogField {
  return {
    id: field.key,
    label: field.label,
    type:
      field.type === 'email' ||
      field.type === 'tel' ||
      field.type === 'number' ||
      field.type === 'textarea' ||
      field.type === 'datetime-local' ||
      field.type === 'select' ||
      field.type === 'checkbox'
        ? field.type
        : 'text',
    placeholder: field.placeholder,
    required: field.required,
    defaultValue: values[field.key] ?? emptyValueForField(field),
    options: field.options,
  };
}

function buildEmptyFormValues(fields: CrudFormField[]): CrudFormValues {
  return Object.fromEntries(fields.map((field) => [field.key, emptyValueForField(field)]));
}

function activeFields(fields: CrudFormField[], editing: boolean) {
  return fields.filter((field) => {
    if (editing && field.createOnly) return false;
    if (!editing && field.editOnly) return false;
    return true;
  });
}

function normalizeError(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}

function pickStringValue(row: Record<string, unknown>, candidates: string[]) {
  for (const key of candidates) {
    const raw = row[key];
    if (typeof raw === 'string' && raw.trim()) return raw.trim();
    if (typeof raw === 'number' && Number.isFinite(raw)) return String(raw);
  }
  return '';
}

export function PymesSimpleCrudListModeContent<T extends { id: string }>({
  resourceId,
  mode = 'list',
  onRowClick,
}: {
  resourceId: string;
  mode?: CrudViewModeId;
  onRowClick?: (row: T) => void;
}) {
  const { t } = useI18n();
  const crudConfigQuery = usePymesCrudConfigQuery<T>(resourceId);
  const crudConfig = crudConfigQuery.data as CrudPageConfig<T> | null;
  const { archived } = useCrudArchivedSearchParam();

  const fetchPage = useCallback(
    async ({
      limit,
      search,
      archived: pageArchived,
      after,
      signal: _signal,
    }: {
      limit: number;
      search: string;
      archived: boolean;
      after: string | null;
      signal: AbortSignal;
    }) => {
      void _signal;
      if (!crudConfig?.basePath) return { items: [], has_more: false, next_cursor: null };
      const query = new URLSearchParams(crudConfig.listQuery ?? '');
      query.set('limit', String(limit));
      if (search) query.set('search', search);
      if (pageArchived) query.set('archived', 'true');
      if (after) query.set('after', after);
      const data = await apiRequest<unknown>(`${crudConfig.basePath}?${query.toString()}`);
      const page = parsePaginatedResponse<T>(data);
      return { items: page.items, has_more: page.hasMore, next_cursor: page.nextCursor } satisfies CrudListResponse<T>;
    },
    [crudConfig],
  );

  const {
    items,
    loading,
    error,
    setError,
    hasMore,
    loadingMore,
    loadMore,
    search: remoteSearch,
    setSearch: setRemoteSearch,
    selectedId,
    selectItem,
    reload,
    handleArchiveToggle,
  } = useCrudRemoteGalleryPage<T>({
    pageSize: 100,
    fetchPage,
  });

  const { search, setSearch, visibleItems, headerLeadSlot, searchInlineActions } = usePymesCrudHeaderFeatures<T>({
    resourceId,
    crudConfigOverride: crudConfig,
    items,
    search: remoteSearch,
    setSearch: setRemoteSearch,
    matchesSearch: (row, query) => {
      const searchText = crudConfig?.searchText?.(row) ?? '';
      return String(searchText).toLowerCase().includes(query);
    },
  });

  const columns = useMemo<CrudTableSurfaceColumn<T>[]>(() => {
    if (!crudConfig) return [];
    const tagsEnabled = crudConfig.featureFlags?.tagsColumn !== false;
    const sourceColumns = archived && crudConfig.archivedColumns?.length ? crudConfig.archivedColumns : crudConfig.columns;
    const mappedColumns: CrudTableSurfaceColumn<T>[] = sourceColumns
      .filter((column) => tagsEnabled || column.key !== 'tags')
      .map((column: CrudColumn<T>) => ({
        id: column.key,
        header: column.header,
        className: column.className,
        render: (row: T) => {
          const value = row[column.key];
          return column.render ? column.render(value, row) : String(value ?? '—');
        },
      }));

    if (
      tagsEnabled &&
      crudConfig.renderTagsCell &&
      !mappedColumns.some((column) => column.id === 'tags')
    ) {
      mappedColumns.push({
        id: 'tags',
        header: 'Tags',
        className: 'cell-tags',
        render: (row) => crudConfig.renderTagsCell?.(row) ?? '—',
      });
    }

    return mappedColumns;
  }, [archived, crudConfig]);

  const runCreateOrEdit = useCallback(
    async (row?: T) => {
      if (!crudConfig) return;
      const editing = Boolean(row);
      const fields = activeFields(crudConfig.formFields, editing);
      if (fields.length === 0) return;
      const values = await openCrudFormDialog({
        title: editing ? `Editar ${crudConfig.label}` : crudConfig.createLabel ?? `Nuevo ${crudConfig.label}`,
        subtitle: editing ? crudConfig.labelPluralCap : undefined,
        submitLabel: editing ? 'Guardar' : 'Crear',
        fields: fields.map((field) =>
          toDialogField(field, editing && row ? crudConfig.toFormValues(row) : buildEmptyFormValues(fields)),
        ),
      });
      if (!values) return;
      if (!crudConfig.isValid(values)) {
        setError(`Revisá los campos de ${crudConfig.label}.`);
        return;
      }

      try {
        if (editing && row) {
          if (crudConfig.dataSource?.update) {
            await crudConfig.dataSource.update(row, values);
          } else if (crudConfig.basePath) {
            await apiRequest(crudItemPath(crudConfig.basePath, row.id), {
              method: 'PUT',
              body: crudConfig.toBody ? crudConfig.toBody(values) : (values as Record<string, unknown>),
            });
          }
        } else if (crudConfig.dataSource?.create) {
          await crudConfig.dataSource.create(values);
        } else if (crudConfig.basePath) {
          await apiRequest(crudConfig.basePath, {
            method: 'POST',
            body: crudConfig.toBody ? crudConfig.toBody(values) : (values as Record<string, unknown>),
          });
        }
        await reload();
      } catch (submitError) {
        setError(normalizeError(submitError, `No se pudo guardar ${crudConfig.label}.`));
      }
    },
    [crudConfig, reload, setError],
  );

  const rowActions = useMemo<CrudTableSurfaceRowAction<T>[]>(() => {
    if (!crudConfig) return [];
    const canEdit = crudConfig.allowEdit ?? Boolean(crudConfig.formFields.length > 0);
    const canDelete = crudConfig.allowDelete ?? Boolean(crudConfig.supportsArchived);
    const canRestore = crudConfig.allowRestore ?? Boolean(crudConfig.supportsArchived);
    const canHardDelete = crudConfig.allowHardDelete ?? Boolean(crudConfig.supportsArchived);
    const helpers: CrudHelpers<T> = {
      items,
      reload,
      setError: (message: string) => setError(message),
    };
    const configRowActions: CrudTableSurfaceRowAction<T>[] = (crudConfig.rowActions ?? []).map((action: CrudRowAction<T>) => ({
      id: action.id,
      label: action.label,
      kind: action.kind,
      isVisible: (row) => action.isVisible?.(row, { archived }) ?? true,
      onClick: async (row) => {
        try {
          await action.onClick(row, helpers);
        } catch (actionError) {
          setError(normalizeError(actionError, `No se pudo ejecutar ${action.label}.`));
        }
      },
    }));
    if (archived) {
      return [
        ...configRowActions,
        ...(canRestore
          ? [
              {
                id: 'restore',
                label: 'Restaurar',
                kind: 'success' as const,
                onClick: async (row: T) => {
                  try {
                    if (crudConfig.dataSource?.restore) {
                      await crudConfig.dataSource.restore(row);
                    } else if (crudConfig.basePath) {
                      await apiRequest(crudItemPath(crudConfig.basePath, row.id, 'restore'), { method: 'POST', body: {} });
                    }
                    await reload();
                  } catch (actionError) {
                    setError(normalizeError(actionError, `No se pudo restaurar ${crudConfig.label}.`));
                  }
                },
              },
            ]
          : []),
        ...(canHardDelete
          ? [
              {
                id: 'hard-delete',
                label: 'Eliminar',
                kind: 'danger' as const,
                onClick: async (row: T) => {
                  try {
                    if (crudConfig.dataSource?.hardDelete) {
                      await crudConfig.dataSource.hardDelete(row);
                    } else if (crudConfig.basePath) {
                      await apiRequest(crudItemPath(crudConfig.basePath, row.id, 'hard'), { method: 'DELETE' });
                    }
                    await reload();
                  } catch (actionError) {
                    setError(normalizeError(actionError, `No se pudo eliminar ${crudConfig.label}.`));
                  }
                },
              },
            ]
          : []),
      ];
    }
    return [
      ...configRowActions,
      ...(canEdit
        ? [
            {
              id: 'edit',
              label: 'Editar',
              onClick: async (row: T) => {
                await runCreateOrEdit(row);
              },
            },
          ]
        : []),
      ...(canDelete
        ? [
            {
              id: 'archive',
              label: 'Archivar',
              kind: 'danger' as const,
              isVisible: () => Boolean(crudConfig.supportsArchived),
              onClick: async (row: T) => {
                try {
                  if (crudConfig.dataSource?.deleteItem) {
                    await crudConfig.dataSource.deleteItem(row);
                  } else if (crudConfig.basePath) {
                    await apiRequest(crudItemPath(crudConfig.basePath, row.id), { method: 'DELETE' });
                  }
                  await reload();
                } catch (actionError) {
                  setError(normalizeError(actionError, `No se pudo archivar ${crudConfig.label}.`));
                }
              },
            },
          ]
        : []),
    ];
  }, [archived, crudConfig, items, reload, runCreateOrEdit, setError]);

  const canEdit = crudConfig?.allowEdit ?? Boolean(crudConfig?.formFields.length);
  const canCreate = crudConfig?.allowCreate ?? Boolean(crudConfig?.formFields.length);
  const rowRecordValues = (row: T) => row as Record<string, unknown>;
  const cardTitle = (row: T) => pickStringValue(rowRecordValues(row), ['name', 'number', 'description', 'title', 'code', 'id']) || row.id;
  const cardSubtitle = (row: T) =>
    pickStringValue(rowRecordValues(row), ['status', 'customer_name', 'supplier_name', 'contact_name', 'category', 'type']);
  const cardMeta = (row: T) =>
    pickStringValue(rowRecordValues(row), ['total', 'amount', 'price', 'created_at', 'valid_until', 'next_due_date']);

  if (!crudConfig) {
    return (
      <div className="empty-state">
        <p>Cargando configuración…</p>
      </div>
    );
  }

  return (
    <div className="products-crud-page">
      <PymesCrudResourceShellHeader<T>
        resourceId={resourceId}
        headerLeadSlot={headerLeadSlot}
        items={visibleItems}
        subtitleCount={visibleItems.length}
        loading={loading}
        error={error}
        setError={setError}
        reload={reload}
        searchValue={search}
        onSearchChange={setSearch}
        onArchiveToggle={handleArchiveToggle}
        searchInlineActions={searchInlineActions}
        extraHeaderActions={
          !archived && canCreate ? (
            <button type="button" className="btn-primary btn-sm" onClick={() => void runCreateOrEdit()}>
              {crudConfig.createLabel ?? `+ Nuevo ${crudConfig.label}`}
            </button>
          ) : null
        }
      />

      {loading ? (
        <div className="empty-state">
          <p>{t('crud.viewMode.gallery.loading')}</p>
        </div>
      ) : visibleItems.length === 0 ? (
        <div className="empty-state">
          <p>{archived ? crudConfig.archivedEmptyState ?? 'No hay archivados para mostrar.' : crudConfig.emptyState ?? 'No hay datos para mostrar.'}</p>
        </div>
      ) : mode === 'gallery' ? (
        <CrudGallerySurface
          items={visibleItems}
          loading={loading}
          emptyLabel={crudConfig.emptyState ?? 'No hay datos para mostrar.'}
          loadingLabel={t('crud.viewMode.gallery.loading')}
          ariaLabel={crudConfig.labelPluralCap}
          card={{
            title: cardTitle,
            subtitle: (row) => cardSubtitle(row),
            meta: (row) => cardMeta(row),
          }}
          onSelect={(row) => {
            if (!archived && canEdit) {
              void runCreateOrEdit(row);
              return;
            }
            selectItem(row.id);
          }}
        />
      ) : mode === 'kanban' ? (
        <CrudValueKanbanSurface<T>
          items={visibleItems}
          loading={loading}
          title={crudConfig.labelPluralCap}
          emptyLabel={archived ? crudConfig.archivedEmptyState ?? 'No hay archivados para mostrar.' : crudConfig.emptyState ?? 'No hay datos para mostrar.'}
          valueFilterOptions={crudConfig.valueFilterOptions}
          onCardOpen={(row) => {
            if (!archived && canEdit) {
              void runCreateOrEdit(row);
              return;
            }
            selectItem(row.id);
          }}
          getCardTitle={cardTitle}
          getCardSubtitle={cardSubtitle}
          getCardMeta={cardMeta}
          disableDrag={archived}
        />
      ) : (
        <CrudTableSurface
          items={visibleItems}
          columns={columns}
          rowActions={rowActions}
          onRowClick={
            onRowClick
              ? onRowClick
              : archived || !canEdit
                ? undefined
                : (row) => {
                    void runCreateOrEdit(row);
                  }
          }
        />
      )}

      {!loading && hasMore ? (
        <div className="crud-load-more">
          <button
            type="button"
            className="btn-secondary"
            disabled={loadingMore}
            onClick={() => {
              void loadMore();
            }}
          >
            {loadingMore ? t('crud.viewMode.gallery.loading') : t('crud.loadMore')}
          </button>
        </div>
      ) : null}
    </div>
  );
}
