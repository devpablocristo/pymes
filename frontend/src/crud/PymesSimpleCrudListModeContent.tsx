import { parsePaginatedResponse } from '@devpablocristo/core-browser/crud';
import { crudItemPath, type CrudFieldValue } from '@devpablocristo/modules-crud-ui';
import { useCallback, useMemo, useState } from 'react';
import './PymesSimpleCrudListModeContent.css';
import { apiRequest } from '../lib/api';
import { useI18n } from '../lib/i18n';
import { PymesCrudResourceShellHeader } from './PymesCrudResourceShellHeader';
import { usePymesCrudConfigQuery } from './usePymesCrudConfigQuery';
import { usePymesCrudHeaderFeatures } from './usePymesCrudHeaderFeatures';
import {
  CrudGallerySurface,
  CrudKanbanColumnCreateButton,
  CrudPaginationBar,
  CrudTableSurface,
  CrudValueKanbanSurface,
  collectCrudImageUrls,
  getCrudStateMachineColumnDefaultState,
  openCrudFormDialog,
  resolveCrudValueFilterOptions,
  useCrudArchivedSearchParam,
  useCrudConfiguredValueKanban,
  useCrudRemoteGalleryPage,
  type CrudActionDialogField,
  type CrudEntityEditorModalBlock,
  type CrudEntityEditorModalSection,
  type CrudEntityEditorModalStat,
  type CrudTableSurfaceColumn,
} from '../modules/crud';
import type {
  CrudColumn,
  CrudEditorModalFieldConfig,
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

function resolveEditorFieldConfig(
  field: CrudFormField,
  overrides?: CrudEditorModalFieldConfig,
  fallbackSectionId?: string,
): CrudEditorModalFieldConfig {
  return {
    sectionId: overrides?.sectionId ?? fallbackSectionId,
    helperText: overrides?.helperText,
    fullWidth: overrides?.fullWidth ?? field.fullWidth,
    hidden: overrides?.hidden,
    readOnly: overrides?.readOnly,
  };
}

function toDialogField(
  field: CrudFormField,
  values: CrudFormValues,
  editorFieldConfig?: CrudEditorModalFieldConfig,
  fallbackSectionId?: string,
): CrudActionDialogField {
  const resolvedEditorFieldConfig = resolveEditorFieldConfig(field, editorFieldConfig, fallbackSectionId);
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
    rows: field.rows,
    defaultValue: values[field.key] ?? emptyValueForField(field),
    options: field.options,
    sectionId: resolvedEditorFieldConfig.sectionId,
    helperText: resolvedEditorFieldConfig.helperText,
    fullWidth: resolvedEditorFieldConfig.fullWidth,
    readOnly: resolvedEditorFieldConfig.readOnly,
    editControl: resolvedEditorFieldConfig.editControl
      ? ({ value, values: dialogValues, setValue }) =>
          resolvedEditorFieldConfig.editControl?.({ value, values: dialogValues, setValue })
      : undefined,
    visible: resolvedEditorFieldConfig.visible
      ? ({ value, values: dialogValues, editing }) =>
          Boolean(resolvedEditorFieldConfig.visible?.({ value, values: dialogValues, editing }))
      : undefined,
    readValue: resolvedEditorFieldConfig.readValue
      ? ({ value, values: dialogValues }) => resolvedEditorFieldConfig.readValue?.({ value, values: dialogValues })
      : undefined,
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

function buildEditorSections<T extends { id: string }>(
  crudConfig: CrudPageConfig<T>,
): CrudEntityEditorModalSection[] | undefined {
  return crudConfig.editorModal?.sections?.map((section) => ({
    id: section.id,
    title: section.title,
    description: section.description,
  }));
}

function resolveEditorSectionId<T extends { id: string }>(
  crudConfig: CrudPageConfig<T>,
  fieldKey: string,
): string | undefined {
  return crudConfig.editorModal?.sections?.find((section) => section.fieldKeys?.includes(fieldKey))?.id;
}

function buildEditorBlocks<T extends { id: string }>(
  crudConfig: CrudPageConfig<T>,
): CrudEntityEditorModalBlock[] | undefined {
  return crudConfig.editorModal?.blocks?.map((block) => ({
    id: block.id,
    kind: block.kind,
    field: block.field,
    sectionId: block.sectionId,
    label: block.label,
    required: block.required,
    visible: block.visible
      ? ({ values, editing, row }) => Boolean(block.visible?.({ values, editing, row: row as T | undefined }))
      : undefined,
  }));
}

function buildEditorStats<T extends { id: string }>(
  crudConfig: CrudPageConfig<T>,
  row: T | undefined,
  initialValues: CrudFormValues,
  editing: boolean,
): CrudEntityEditorModalStat[] | undefined {
  return crudConfig.editorModal?.stats?.map((stat) => ({
    id: stat.id,
    label: stat.label,
    tone: stat.tone,
    value: (values) => stat.value({ row, values: values as CrudFormValues, editing }),
  }));
}

function pickStringValue(row: Record<string, unknown>, candidates: string[]) {
  for (const key of candidates) {
    const raw = row[key];
    if (typeof raw === 'string' && raw.trim()) return raw.trim();
    if (typeof raw === 'number' && Number.isFinite(raw)) return String(raw);
  }
  return '';
}

function buildEditorMediaUrls<T extends { id: string }>(row: T | undefined) {
  if (!row) return undefined;
  const record = row as Record<string, unknown>;
  return collectCrudImageUrls({
    imageUrls: Array.isArray(record.image_urls)
      ? record.image_urls.filter((value): value is string => typeof value === 'string')
      : Array.isArray(record.imageUrls)
        ? record.imageUrls.filter((value): value is string => typeof value === 'string')
        : undefined,
    legacyImageUrl:
      typeof record.image_url === 'string'
        ? record.image_url
        : typeof record.imageUrl === 'string'
          ? record.imageUrl
          : undefined,
  });
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
    total,
    hasMore,
    loadingMore,
    loadMore,
    setItems,
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
  const resolvedValueFilterOptions = resolveCrudValueFilterOptions(crudConfig);

  const valueKanban = useCrudConfiguredValueKanban<T>({
    crudConfig,
    items,
    setItems,
    reload,
    setError,
    archived,
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
    async (row?: T, createDefaults: CrudFormValues = {}) => {
      if (!crudConfig) return;
      const editing = Boolean(row);
      let editorRow = row;
      if (editing && row && crudConfig.editorModal?.loadRecord) {
        try {
          editorRow = await crudConfig.editorModal.loadRecord(row);
        } catch (loadError) {
          setError(normalizeError(loadError, `No se pudo cargar ${crudConfig.label}.`));
          return;
        }
      }
      const fields = activeFields(crudConfig.formFields, editing);
      const blocks = buildEditorBlocks(crudConfig);
      if (fields.length === 0 && !(blocks?.length)) return;
      const createInitialValues = {
        ...buildEmptyFormValues(fields),
        ...createDefaults,
      };
      const currentValues = editing && editorRow ? crudConfig.toFormValues(editorRow) : createInitialValues;
      const dialogTitle =
        editing && editorRow
          ? pickStringValue(editorRow as Record<string, unknown>, ['number', 'name', 'title']) || `Detalle de ${crudConfig.label}`
          : crudConfig.createLabel ?? `Nuevo ${crudConfig.label}`;
      const visibleFields = fields.filter(
        (field) => !crudConfig.editorModal?.fieldConfig?.[field.key]?.hidden,
      );
      const values = await openCrudFormDialog({
        title: editing ? '' : dialogTitle,
        subtitle: undefined,
        eyebrow: editing ? undefined : crudConfig.editorModal?.eyebrow ?? crudConfig.labelPluralCap,
        mediaUrls: editing ? buildEditorMediaUrls(editorRow) : undefined,
        mediaFieldId: crudConfig.editorModal?.mediaFieldKey,
        dialogMode: editing ? 'update' : 'create',
        submitLabel: editing ? 'Guardar' : 'Crear',
        editLabel: 'Editar',
        cancelEditLabel: 'Cancelar edición',
        closeLabel: 'Cerrar',
        initialValues: currentValues,
        fields: visibleFields.map((field) =>
          toDialogField(
            field,
            currentValues,
            crudConfig.editorModal?.fieldConfig?.[field.key],
            resolveEditorSectionId(crudConfig, field.key),
          ),
        ),
        blocks,
        sections: buildEditorSections(crudConfig),
        stats: buildEditorStats(crudConfig, editorRow, currentValues, editing),
        row: editorRow,
        confirmDiscard: crudConfig.editorModal?.confirmDiscard,
        archiveAction:
          editing && editorRow && crudConfig.supportsArchived
            ? {
                label: 'Archivar',
                busyLabel: 'Archivando…',
                confirm: {
                  title: `Archivar ${crudConfig.label}`,
                  description: `Este ${crudConfig.label} va a dejar de verse en los listados activos.`,
                  confirmLabel: 'Archivar',
                  cancelLabel: 'Cancelar',
                },
                onArchive: async () => {
                  try {
                    if (crudConfig.dataSource?.deleteItem) {
                      await crudConfig.dataSource.deleteItem(editorRow);
                    } else if (crudConfig.basePath) {
                      await apiRequest(crudItemPath(crudConfig.basePath, editorRow.id), { method: 'DELETE' });
                    }
                    await reload();
                  } catch (archiveError) {
                    setError(normalizeError(archiveError, `No se pudo archivar ${crudConfig.label}.`));
                    throw archiveError;
                  }
                },
              }
            : undefined,
      });
      if (!values) return;
      const submittedValues = editing ? { ...currentValues, ...values } : { ...createInitialValues, ...values };
      if (!crudConfig.isValid(submittedValues)) {
        setError(`Revisá los campos de ${crudConfig.label}.`);
        return;
      }

      try {
        if (editing && row) {
          if (crudConfig.dataSource?.update) {
            await crudConfig.dataSource.update(row, submittedValues);
          } else if (crudConfig.basePath) {
            await apiRequest(crudItemPath(crudConfig.basePath, row.id), {
              method: 'PUT',
              body: crudConfig.toBody ? crudConfig.toBody(submittedValues) : (submittedValues as Record<string, unknown>),
            });
          }
        } else if (crudConfig.dataSource?.create) {
          await crudConfig.dataSource.create(submittedValues);
        } else if (crudConfig.basePath) {
          await apiRequest(crudConfig.basePath, {
            method: 'POST',
            body: crudConfig.toBody ? crudConfig.toBody(submittedValues) : (submittedValues as Record<string, unknown>),
          });
        }
        await reload();
      } catch (submitError) {
        setError(normalizeError(submitError, `No se pudo guardar ${crudConfig.label}.`));
      }
    },
    [crudConfig, reload, setError],
  );

  const canEdit = crudConfig?.allowEdit ?? Boolean(crudConfig?.formFields.length);
  const canCreate = crudConfig?.allowCreate ?? Boolean(crudConfig?.formFields.length);
  const paginationEnabled = crudConfig?.featureFlags?.pagination !== false;
  const kanbanCreateFooterLabel = crudConfig?.kanban?.createFooterLabel ?? `Añadir ${crudConfig?.label ?? 'registro'}`;
  const getKanbanCreateDefaults = useCallback(
    (columnId: string): CrudFormValues => {
      if (!crudConfig?.stateMachine) return {};
      const nextState = getCrudStateMachineColumnDefaultState(crudConfig.stateMachine, columnId);
      if (!nextState) return {};
      return { [crudConfig.stateMachine.field]: nextState };
    },
    [crudConfig],
  );

  const resolvedTableRowClick = useMemo(() => {
    if (onRowClick) return onRowClick;
    if (archived || !canEdit) return undefined;
    return (row: T) => {
      void runCreateOrEdit(row);
    };
  }, [archived, canEdit, onRowClick, runCreateOrEdit]);

  const rowRecordValues = (row: T) => row as Record<string, unknown>;
  const kanbanCardConfig = crudConfig?.kanban?.card;
  const cardTitle = (row: T) =>
    (
      kanbanCardConfig?.title?.(row) ??
      pickStringValue(rowRecordValues(row), ['name', 'number', 'description', 'title', 'code', 'id'])
    ) || row.id;
  const cardSubtitle = (row: T) => {
    if (kanbanCardConfig?.subtitle) {
      return kanbanCardConfig.subtitle(row);
    }
    const stateField = crudConfig?.stateMachine?.field;
    const candidates = ['status', 'customer_name', 'supplier_name', 'contact_name', 'category', 'type'].filter(
      (key) => key !== stateField,
    );
    return pickStringValue(rowRecordValues(row), candidates);
  };
  const cardMeta = (row: T) =>
    kanbanCardConfig?.meta?.(row) ??
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
          !archived && canCreate && crudConfig.featureFlags?.createAction !== false ? (
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
          stateMachine={crudConfig.stateMachine}
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
          onMoveCard={valueKanban.enabled ? valueKanban.onMoveCard : undefined}
          isRowDraggable={valueKanban.enabled ? valueKanban.isRowDraggable : undefined}
          isColumnDroppable={valueKanban.enabled ? valueKanban.isColumnDroppable : undefined}
          columnFooter={
            !archived && canCreate
              ? (columnId) => (
                  <CrudKanbanColumnCreateButton
                    label={kanbanCreateFooterLabel}
                    onClick={() => {
                      void runCreateOrEdit(undefined, getKanbanCreateDefaults(columnId));
                    }}
                  />
                )
              : undefined
          }
        />
      ) : (
        <CrudTableSurface
          items={visibleItems}
          columns={columns}
          onRowClick={resolvedTableRowClick}
        />
      )}

      {!loading && visibleItems.length > 0 ? (
        <CrudPaginationBar
          visibleCount={visibleItems.length}
          totalCount={total || items.length}
          hasMore={hasMore}
          loadingMore={loadingMore}
          onLoadMore={() => {
            void loadMore();
          }}
          hidden={!paginationEnabled}
        />
      ) : null}
    </div>
  );
}
