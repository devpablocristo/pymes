import { useMemo } from 'react';
import { apiRequest } from '../../lib/api';
import { formatDate } from '../../crud/resourceConfigs.shared';
import { PymesCrudResourceShellHeader } from '../../crud/PymesCrudResourceShellHeader';
import { usePymesCrudHeaderFeatures } from '../../crud/usePymesCrudHeaderFeatures';
import {
  CrudTableSurface,
  buildCrudContextEntityPath,
  getCrudContextEntityParams,
  openCrudFormDialog,
  useCrudRemoteListState,
  type CrudTableSurfaceColumn,
} from '../crud';
import type { TimelineEntryRow } from './auditTrailHelpers';

export function TimelineListModeContent() {
  const context = getCrudContextEntityParams();
  const listPath = buildCrudContextEntityPath(context, '/timeline?limit=100');
  const notePath = buildCrudContextEntityPath(context, '/notes');

  const { items, error, setError, loading, reload } = useCrudRemoteListState<TimelineEntryRow>({
    queryKey: ['timeline', context.entity ?? 'none', context.entityId ?? 'none'],
    list: async () => {
      if (!listPath) return [];
      const data = await apiRequest<{ items?: TimelineEntryRow[] | null }>(listPath);
      return (data.items ?? []).map((row) => ({ ...row, id: String(row.id) }));
    },
    loadErrorMessage: 'No se pudo cargar el historial.',
  });

  const { search, setSearch, visibleItems, headerLeadSlot, searchInlineActions } = usePymesCrudHeaderFeatures<TimelineEntryRow>({
    resourceId: 'timeline',
    items,
    matchesSearch: (row, query) =>
      [row.title, row.description, row.event_type, row.actor, row.entity_type]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(query),
  });

  const columns = useMemo<CrudTableSurfaceColumn<TimelineEntryRow>[]>(
    () => [
      {
        id: 'title',
        header: 'Evento',
        className: 'cell-name',
        render: (row) => (
          <>
            <strong>{row.title}</strong>
            <div className="text-secondary">{row.event_type}</div>
          </>
        ),
      },
      { id: 'description', header: 'Detalle', className: 'cell-notes', render: (row) => row.description || '—' },
      { id: 'actor', header: 'Actor', render: (row) => row.actor || '—' },
      { id: 'created_at', header: 'Fecha', render: (row) => formatDate(String(row.created_at ?? '')) },
    ],
    [],
  );

  async function handleCreateNote() {
    if (!notePath) {
      setError('Faltan entity y entity_id en la URL.');
      return;
    }
    const values = await openCrudFormDialog({
      title: 'Nueva nota manual',
      submitLabel: 'Guardar nota',
      fields: [
        { id: 'title', label: 'Título', placeholder: 'Nota manual' },
        { id: 'note', label: 'Nota', type: 'textarea', required: true, rows: 5 },
      ],
    });
    if (!values) return;
    if (!String(values.note ?? '').trim()) return;

    await apiRequest(notePath, {
      method: 'POST',
      body: {
        title: String(values.title ?? '').trim() || undefined,
        note: String(values.note ?? '').trim(),
      },
    });
    await reload();
  }

  return (
    <div className="products-crud-page">
      <PymesCrudResourceShellHeader<TimelineEntryRow>
        resourceId="timeline"
        items={visibleItems}
        subtitleCount={visibleItems.length}
        loading={loading}
        error={error}
        setError={setError}
        reload={reload}
        searchValue={search}
        onSearchChange={setSearch}
        headerLeadSlot={headerLeadSlot}
        searchInlineActions={searchInlineActions}
        extraHeaderActions={
          <button type="button" className="btn-primary btn-sm" onClick={() => void handleCreateNote()}>
            + Nota manual
          </button>
        }
      />
      {loading ? (
        <div className="empty-state">
          <p>Cargando historial…</p>
        </div>
      ) : visibleItems.length === 0 ? (
        <div className="empty-state">
          <p>{listPath ? 'No hay entradas para mostrar.' : 'Agregá entity y entity_id en la URL.'}</p>
        </div>
      ) : (
        <CrudTableSurface items={visibleItems} columns={columns} />
      )}
    </div>
  );
}
