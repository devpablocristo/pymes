import { useMemo, useState } from 'react';
import { apiRequest } from '../../lib/api';
import { formatDate } from '../../crud/resourceConfigs.shared';
import { PymesCrudResourceShellHeader } from '../../crud/PymesCrudResourceShellHeader';
import { CrudTableSurface, useCrudRemoteListState, type CrudTableSurfaceColumn } from '../crud';
import type { AuditEntryRow } from './auditTrailHelpers';

export function AuditListModeContent() {
  const [search, setSearch] = useState('');
  const { items, error, setError, loading, reload } = useCrudRemoteListState<AuditEntryRow>({
    queryKey: ['audit'],
    list: async () => {
      const data = await apiRequest<{ items?: AuditEntryRow[] | null }>('/v1/audit');
      return (data.items ?? []).map((row) => ({ ...row, id: String(row.id) }));
    },
    loadErrorMessage: 'No se pudo cargar la auditoría.',
  });

  const visibleItems = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return items;
    return items.filter((row) =>
      [row.action, row.resource_type, row.resource_id, row.actor, row.actor_label].filter(Boolean).join(' ').toLowerCase().includes(q),
    );
  }, [items, search]);

  const columns = useMemo<CrudTableSurfaceColumn<AuditEntryRow>[]>(
    () => [
      {
        id: 'action',
        header: 'Acción',
        className: 'cell-name',
        render: (row) => (
          <>
            <strong>{row.action}</strong>
            <div className="text-secondary">{row.resource_type}</div>
          </>
        ),
      },
      { id: 'resource_id', header: 'Recurso', render: (row) => row.resource_id || '—' },
      { id: 'actor', header: 'Actor', render: (row) => row.actor_label || row.actor || '—' },
      { id: 'created_at', header: 'Fecha', render: (row) => formatDate(String(row.created_at ?? '')) },
    ],
    [],
  );

  return (
    <div className="products-crud-page">
      <PymesCrudResourceShellHeader<AuditEntryRow>
        resourceId="audit"
        preserveCsvToolbar
        items={items}
        subtitleCount={visibleItems.length}
        loading={loading}
        error={error}
        setError={setError}
        reload={reload}
        searchValue={search}
        onSearchChange={setSearch}
      />
      {loading ? (
        <div className="empty-state">
          <p>Cargando auditoría…</p>
        </div>
      ) : visibleItems.length === 0 ? (
        <div className="empty-state">
          <p>No hay eventos para mostrar.</p>
        </div>
      ) : (
        <CrudTableSurface items={visibleItems} columns={columns} />
      )}
    </div>
  );
}
