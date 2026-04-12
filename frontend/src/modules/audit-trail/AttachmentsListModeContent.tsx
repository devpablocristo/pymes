import { useMemo, useState } from 'react';
import { apiRequest, downloadAPIFile } from '../../lib/api';
import { formatDate } from '../../crud/resourceConfigs.shared';
import { PymesCrudResourceShellHeader } from '../../crud/PymesCrudResourceShellHeader';
import {
  CrudTableSurface,
  buildCrudContextEntityPath,
  getCrudContextEntityParams,
  openCrudTextDialog,
  useCrudRemoteListState,
  type CrudTableSurfaceColumn,
  type CrudTableSurfaceRowAction,
} from '../crud';
import type { AttachmentRow } from './auditTrailHelpers';

export function AttachmentsListModeContent() {
  const [search, setSearch] = useState('');
  const context = getCrudContextEntityParams();
  const path = buildCrudContextEntityPath(context, '/attachments?limit=200');

  const { items, setItems, error, setError, loading, reload } = useCrudRemoteListState<AttachmentRow>({
    queryKey: ['attachments', context.entity ?? 'none', context.entityId ?? 'none'],
    list: async () => {
      if (!path) return [];
      const data = await apiRequest<{ items?: AttachmentRow[] | null }>(path);
      return (data.items ?? []).map((row) => ({ ...row, id: String(row.id) }));
    },
    loadErrorMessage: 'No se pudieron cargar los adjuntos.',
  });

  const visibleItems = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return items;
    return items.filter((row) =>
      [row.file_name, row.content_type, row.uploaded_by, String(row.size_bytes)].filter(Boolean).join(' ').toLowerCase().includes(q),
    );
  }, [items, search]);

  const columns = useMemo<CrudTableSurfaceColumn<AttachmentRow>[]>(
    () => [
      {
        id: 'file_name',
        header: 'Archivo',
        className: 'cell-name',
        render: (row) => (
          <>
            <strong>{row.file_name}</strong>
            <div className="text-secondary">{row.content_type}</div>
          </>
        ),
      },
      { id: 'size_bytes', header: 'Tamaño', render: (row) => String(row.size_bytes ?? '') || '—' },
      { id: 'uploaded_by', header: 'Subido por', render: (row) => row.uploaded_by || '—' },
      { id: 'created_at', header: 'Fecha', render: (row) => formatDate(String(row.created_at ?? '')) },
    ],
    [],
  );

  const rowActions = useMemo<CrudTableSurfaceRowAction<AttachmentRow>[]>(
    () => [
      {
        id: 'signed-url',
        label: 'Enlace firmado',
        kind: 'secondary',
        onClick: async (row) => {
          try {
            const link = await apiRequest<{ url: string }>(`/v1/attachments/${row.id}/url`);
            if (link.url) {
              await openCrudTextDialog({ title: 'Enlace firmado', subtitle: row.file_name, textContent: link.url });
            }
          } catch (e) {
            setError(e instanceof Error ? e.message : 'No se pudo obtener el enlace.');
          }
        },
      },
      {
        id: 'download',
        label: 'Descargar',
        kind: 'primary',
        onClick: async (row) => {
          try {
            await downloadAPIFile(`/v1/attachments/${row.id}/download`);
          } catch (e) {
            setError(e instanceof Error ? e.message : 'No se pudo descargar.');
          }
        },
      },
      {
        id: 'delete',
        label: 'Eliminar',
        kind: 'danger',
        onClick: async (row) => {
          try {
            await apiRequest(`/v1/attachments/${row.id}`, { method: 'DELETE' });
            setItems((current) => current.filter((item) => item.id !== row.id));
          } catch (e) {
            setError(e instanceof Error ? e.message : 'No se pudo eliminar.');
          }
        },
      },
    ],
    [setError, setItems],
  );

  return (
    <div className="products-crud-page">
      <PymesCrudResourceShellHeader<AttachmentRow>
        resourceId="attachments"
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
          <p>Cargando adjuntos…</p>
        </div>
      ) : visibleItems.length === 0 ? (
        <div className="empty-state">
          <p>{path ? 'No hay adjuntos para mostrar.' : 'Agregá entity y entity_id en la URL.'}</p>
        </div>
      ) : (
        <CrudTableSurface items={visibleItems} columns={columns} rowActions={rowActions} />
      )}
    </div>
  );
}
