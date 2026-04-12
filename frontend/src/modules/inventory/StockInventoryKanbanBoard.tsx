/**
 * Tablero de stock con la misma superficie y columnas que las órdenes de trabajo (StatusKanbanBoard).
 * El arrastre actualiza solo el tablero en memoria (no hay API de “fase” para inventario); al recargar datos del servidor se revierte.
 */
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { normalize } from '@devpablocristo/core-browser/search';
import type { KanbanColumnDef, SuppressCardOpen } from '@devpablocristo/modules-kanban-board';
import { useCallback, useEffect, useMemo, useState, type RefObject } from 'react';
import { Link } from 'react-router-dom';
import {
  createCrudKanbanArchiveTerminalDragPolicy,
  CrudKanbanSurface,
  useCrudArchivedSearchParam,
} from '../crud';
import { PymesCrudResourceShellHeader } from '../../crud/PymesCrudResourceShellHeader';
import { usePymesCrudHeaderFeatures } from '../../crud/usePymesCrudHeaderFeatures';
import { StockLevelDetailModal } from './StockLevelDetailModal';
import { fetchStockLevels, type StockLevelRow } from './stockData';
import '../../pages/InventoryPage.css';
import '../../pages/WorkOrdersKanbanPanel.css';

const COLUMN_ORDER: KanbanColumnDef[] = [
  { id: 'wo_intake', label: 'Ingreso' },
  { id: 'wo_quote', label: 'Presupuesto / repuestos' },
  { id: 'wo_shop', label: 'Taller' },
  { id: 'wo_exit', label: 'Salida' },
  { id: 'wo_closed', label: 'Cerradas' },
];

const COLUMN_IDS = new Set(COLUMN_ORDER.map((c) => c.id));

export function stockKanbanPhase(row: StockLevelRow): string {
  if (!row.sku?.trim()) return 'wo_intake';
  if (row.quantity < 0) return 'wo_shop';
  if (row.is_low_stock) return 'wo_quote';
  if (row.quantity === 0) return 'wo_closed';
  return 'wo_exit';
}

function displayPhase(row: StockLevelRow, manual: Record<string, string>): string {
  const manualId = manual[row.id];
  if (manualId && COLUMN_IDS.has(manualId)) return manualId;
  return stockKanbanPhase(row);
}

function resolveStockDropColumnId(
  overId: string | undefined,
  items: StockLevelRow[],
  manual: Record<string, string>,
): string | null {
  if (!overId) return null;
  if (overId.startsWith('col-')) {
    const s = overId.slice(4);
    return COLUMN_IDS.has(s) ? s : null;
  }
  const overCard = items.find((x) => x.id === overId);
  if (overCard) {
    const c = displayPhase(overCard, manual);
    return COLUMN_IDS.has(c) ? c : 'wo_intake';
  }
  return null;
}

function StockKanbanCardBody({
  row,
  onOpen,
  suppressOpenRef,
}: {
  row: StockLevelRow;
  onOpen: () => void;
  suppressOpenRef: RefObject<SuppressCardOpen>;
}) {
  const handleClick = () => {
    const s = suppressOpenRef.current;
    if (s != null && s.id === row.id && Date.now() < s.until) return;
    onOpen();
  };
  const badgeClass = row.is_low_stock
    ? 'wo-kanban__badge wo-kanban__badge--hold'
    : row.quantity < 0
      ? 'wo-kanban__badge wo-kanban__badge--terminal-danger'
      : 'wo-kanban__badge';
  const badgeLabel = row.is_low_stock ? 'Bajo mínimo' : row.quantity < 0 ? 'Negativo' : 'Normal';
  return (
    <div
      className="m-kanban__card"
      title="Clic para abrir detalle de stock"
      aria-label={`${row.product_name}. ${badgeLabel}.`}
      draggable={false}
      onClick={handleClick}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          handleClick();
        }
      }}
      role="button"
      tabIndex={0}
    >
      <strong>{row.product_name}</strong>
      <div className="wo-kanban__badges">
        <span className={badgeClass}>{badgeLabel}</span>
      </div>
      <div className="m-kanban__card-meta">
        {row.sku?.trim() || 'sin SKU'} · {row.quantity} u. (mín. {row.min_quantity})
      </div>
    </div>
  );
}

function StockKanbanCardPreview({ row }: { row: StockLevelRow }) {
  const badgeClass = row.is_low_stock
    ? 'wo-kanban__badge wo-kanban__badge--hold'
    : row.quantity < 0
      ? 'wo-kanban__badge wo-kanban__badge--terminal-danger'
      : 'wo-kanban__badge';
  const badgeLabel = row.is_low_stock ? 'Bajo mínimo' : row.quantity < 0 ? 'Negativo' : 'Normal';
  return (
    <div className="m-kanban__card m-kanban__card--overlay" aria-hidden="true">
      <strong>{row.product_name}</strong>
      <div className="wo-kanban__badges" aria-hidden="true">
        <span className={badgeClass}>{badgeLabel}</span>
      </div>
      <div className="m-kanban__card-meta">
        {row.sku?.trim() || '—'} · {row.quantity} u.
      </div>
    </div>
  );
}

export function StockInventoryKanbanBoard() {
  const queryClient = useQueryClient();
  const { archived: showArchived } = useCrudArchivedSearchParam();
  const [detailProductId, setDetailProductId] = useState<string | null>(null);
  const [toolbarError, setToolbarError] = useState<string | null>(null);
  const [manualColumnById, setManualColumnById] = useState<Record<string, string>>({});

  const stockQuery = useQuery<StockLevelRow[]>({
    queryKey: ['inventory', 'inventory-kanban', showArchived ? 'archived' : 'active'],
    queryFn: () => fetchStockLevels({ archived: showArchived }),
    refetchOnWindowFocus: false,
    staleTime: 30_000,
  });

  const items = stockQuery.data ?? [];
  const loadError =
    stockQuery.error instanceof Error ? stockQuery.error.message : stockQuery.error ? String(stockQuery.error) : null;
  const combinedError = loadError ?? toolbarError;

  useEffect(() => {
    if (stockQuery.isSuccess) setToolbarError(null);
  }, [stockQuery.isSuccess, stockQuery.dataUpdatedAt]);

  useEffect(() => {
    setManualColumnById({});
  }, [stockQuery.data]);

  const reload = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: ['inventory', 'inventory-kanban'] });
  }, [queryClient]);

  const getRowColumnId = useCallback((row: StockLevelRow) => displayPhase(row, manualColumnById), [manualColumnById]);

  const archiveTerminalDragPolicy = useMemo(
    () =>
      createCrudKanbanArchiveTerminalDragPolicy<StockLevelRow>({
        showArchived,
        transitionModel: { isTerminalStatus: (phase) => phase === 'wo_closed' },
        getItemStatus: (row) => displayPhase(row, manualColumnById),
      }),
    [manualColumnById, showArchived],
  );

  const handleMoveCard = useCallback((id: string, targetColumnId: string) => {
    if (!COLUMN_IDS.has(targetColumnId)) return;
    setManualColumnById((prev) => ({ ...prev, [id]: targetColumnId }));
  }, []);

  const filterRow = useCallback((row: StockLevelRow, q: string) => {
    const hay = normalize(
      [row.product_name, row.sku, String(row.quantity), String(row.min_quantity), row.is_low_stock ? 'bajo' : 'normal'].join(' '),
    );
    return hay.includes(normalize(q));
  }, []);

  const statsLine = useCallback((_visible: number, _total: number) => '', []);
  const { search, setSearch, visibleItems, headerLeadSlot, searchInlineActions } = usePymesCrudHeaderFeatures<StockLevelRow>({
    resourceId: 'inventory',
    items,
    matchesSearch: filterRow,
  });

  return (
    <>
      <PymesCrudResourceShellHeader<StockLevelRow>
        resourceId="inventory"
        preserveCsvToolbar
        items={visibleItems}
        subtitleCount={visibleItems.length}
        loading={stockQuery.isLoading}
        error={combinedError}
        setError={setToolbarError}
        reload={reload}
        searchValue={search}
        onSearchChange={setSearch}
        onArchiveToggle={() => setDetailProductId(null)}
        headerLeadSlot={headerLeadSlot}
        searchInlineActions={searchInlineActions}
      />
      <div className="stock-inventory-kanban__board-only">
        <CrudKanbanSurface<StockLevelRow>
          columns={COLUMN_ORDER}
          columnIdSet={COLUMN_IDS}
          getRowColumnId={getRowColumnId}
          fallbackColumnId="wo_intake"
          items={visibleItems}
          loading={stockQuery.isLoading}
          error={null}
          onMoveCard={handleMoveCard}
          resolveDropColumnId={(overId, snapshot) => resolveStockDropColumnId(overId, snapshot, manualColumnById)}
          filterRow={filterRow}
          isRowDraggable={archiveTerminalDragPolicy.isRowDraggable}
          isColumnDroppable={archiveTerminalDragPolicy.isColumnDroppable}
          onCardOpen={(row) => setDetailProductId(row.product_id)}
          renderCard={({ row, onOpen, suppressOpenRef }) => (
            <StockKanbanCardBody row={row} onOpen={onOpen} suppressOpenRef={suppressOpenRef} />
          )}
          renderOverlayCard={(row) => <StockKanbanCardPreview row={row} />}
          title="Inventario"
          externalSearch=""
          statsLine={statsLine}
          columnFooter={() =>
            showArchived ? null : (
              <Link to="/modules/products/list" className="m-kanban__column-add" draggable={false}>
                <span className="m-kanban__column-add-icon" aria-hidden="true">
                  +
                </span>
                Añadir producto
              </Link>
            )
          }
        />
      </div>
      <StockLevelDetailModal
        productId={detailProductId}
        onClose={() => setDetailProductId(null)}
        onAfterSave={() => {
          void queryClient.invalidateQueries({ queryKey: ['inventory', 'inventory-kanban'] });
        }}
      />
    </>
  );
}
