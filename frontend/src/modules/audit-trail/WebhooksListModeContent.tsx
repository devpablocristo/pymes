import { useMemo, useState } from 'react';
import { apiRequest } from '../../lib/api';
import { formatDate } from '../../crud/resourceConfigs.shared';
import { PymesCrudResourceShellHeader } from '../../crud/PymesCrudResourceShellHeader';
import { CrudTableSurface, useCrudRemoteListState, type CrudTableSurfaceColumn, type CrudTableSurfaceRowAction } from '../crud';
import type { WebhookEndpoint } from './auditTrailHelpers';

export function WebhooksListModeContent() {
  const [search, setSearch] = useState('');
  const { items, error, setError, loading, reload } = useCrudRemoteListState<WebhookEndpoint>({
    queryKey: ['webhooks'],
    list: async () => {
      const data = await apiRequest<{ items?: WebhookEndpoint[] | null }>('/v1/webhook-endpoints');
      return (data.items ?? []).map((row) => ({ ...row, id: String(row.id) }));
    },
    loadErrorMessage: 'No se pudieron cargar los webhooks.',
  });

  const visibleItems = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return items;
    return items.filter((row) => [row.url, ...(row.events ?? [])].join(' ').toLowerCase().includes(q));
  }, [items, search]);

  const columns = useMemo<CrudTableSurfaceColumn<WebhookEndpoint>[]>(
    () => [
      {
        id: 'url',
        header: 'Endpoint',
        className: 'cell-name',
        render: (row) => (
          <>
            <strong>{row.url}</strong>
            <div className="text-secondary">{(row.events ?? []).join(', ') || 'Sin eventos'}</div>
          </>
        ),
      },
      {
        id: 'is_active',
        header: 'Estado',
        render: (row) => <span className={`badge ${row.is_active ? 'badge-success' : 'badge-neutral'}`}>{row.is_active ? 'Activo' : 'Inactivo'}</span>,
      },
      { id: 'created_at', header: 'Creado', render: (row) => formatDate(String(row.created_at ?? '')) },
      { id: 'secret', header: 'Secret', render: (row) => (String(row.secret ?? '').trim() ? 'Configurado' : '—') },
    ],
    [],
  );

  const rowActions = useMemo<CrudTableSurfaceRowAction<WebhookEndpoint>[]>(
    () => [
      {
        id: 'test',
        label: 'Probar',
        kind: 'success',
        onClick: async (row) => {
          try {
            await apiRequest(`/v1/webhook-endpoints/${row.id}/test`, { method: 'POST', body: {} });
          } catch (e) {
            setError(e instanceof Error ? e.message : 'No se pudo probar.');
          }
        },
      },
    ],
    [setError],
  );

  return (
    <div className="products-crud-page">
      <PymesCrudResourceShellHeader<WebhookEndpoint>
        resourceId="webhooks"
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
          <p>Cargando webhooks…</p>
        </div>
      ) : visibleItems.length === 0 ? (
        <div className="empty-state">
          <p>No hay webhooks para mostrar.</p>
        </div>
      ) : (
        <CrudTableSurface items={visibleItems} columns={columns} rowActions={rowActions} />
      )}
    </div>
  );
}
