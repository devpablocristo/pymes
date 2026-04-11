/**
 * Tablero de stock con la misma superficie y columnas que las órdenes de trabajo (StatusKanbanBoard).
 * El arrastre actualiza solo el tablero en memoria (no hay API de “fase” para inventario); al recargar datos del servidor se revierte.
 */
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { normalize } from '@devpablocristo/core-browser/search';
import type { KanbanColumnDef, SuppressCardOpen } from '@devpablocristo/modules-kanban-board';
import { useCallback, useEffect, useMemo, useState, type ReactElement, type RefObject } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import type { CrudHelpers } from '../../components/CrudPage';
import { CrudKanbanSurface } from '../crud';
import { loadLazyCrudPageConfig } from '../../crud/lazyCrudPage';
import { useI18n } from '../../lib/i18n';
import { StockLevelDetailModal } from './StockLevelDetailModal';
import { fetchStockLevels, type StockLevelRow } from './stockLevels';
import '../../pages/WorkOrdersKanbanPanel.css';

/** Mismas columnas que `GenericWorkOrdersBoard` (órdenes auto). */
const COLUMN_ORDER: KanbanColumnDef[] = [
  { id: 'wo_intake', label: 'Ingreso' },
  { id: 'wo_quote', label: 'Presupuesto / repuestos' },
  { id: 'wo_shop', label: 'Taller' },
  { id: 'wo_exit', label: 'Salida' },
  { id: 'wo_closed', label: 'Cerradas' },
];

const COLUMN_IDS = new Set(COLUMN_ORDER.map((c) => c.id));

/** Reparto en columnas “tipo OT” según señales de inventario (sin persistir movimientos). */
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

function toolbarBtnClass(kind?: 'primary' | 'secondary' | 'danger' | 'success'): string {
  switch (kind) {
    case 'primary':
      return 'btn-sm btn-primary';
    case 'danger':
      return 'btn-sm btn-danger';
    case 'success':
      return 'btn-sm btn-success';
    default:
      return 'btn-sm btn-secondary';
  }
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
  const { localizeText: formatFieldText } = useI18n();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [detailProductId, setDetailProductId] = useState<string | null>(null);
  const [toolbarError, setToolbarError] = useState<string | null>(null);
  /** Columna Kanban elegida por drag (no persiste en API). */
  const [manualColumnById, setManualColumnById] = useState<Record<string, string>>({});

  const stockQuery = useQuery({
    queryKey: ['stock', 'inventory-kanban'],
    queryFn: fetchStockLevels,
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
    await queryClient.invalidateQueries({ queryKey: ['stock', 'inventory-kanban'] });
  }, [queryClient]);

  const getRowColumnId = useCallback(
    (row: StockLevelRow) => displayPhase(row, manualColumnById),
    [manualColumnById],
  );

  const handleMoveCard = useCallback((id: string, targetColumnId: string) => {
    if (!COLUMN_IDS.has(targetColumnId)) return;
    setManualColumnById((prev) => ({ ...prev, [id]: targetColumnId }));
  }, []);

  const crudConfigQuery = useQuery({
    queryKey: ['stock', 'crud-config'],
    queryFn: () => loadLazyCrudPageConfig<StockLevelRow>('stock'),
  });
  const crudConfig = crudConfigQuery.data ?? null;

  const renderExtraToolbar = useMemo(
    () =>
      ({
        items: toolbarItems,
        reload: toolbarReload,
        setError: toolbarSetError,
      }: {
        items: StockLevelRow[];
        reload: () => Promise<void>;
        setError: (message: string | null) => void;
      }): ReactElement => {
        const helpers: CrudHelpers<StockLevelRow> = {
          items: toolbarItems,
          reload: toolbarReload,
          setError: (message: string) => toolbarSetError(message),
        };
        const toolbarActions = (crudConfig?.toolbarActions ?? []).filter(
          (action) => action.isVisible?.({ archived: false, items: toolbarItems }) ?? true,
        );
        return (
          <>
            {toolbarActions.map((action) => (
              <button
                key={action.id}
                type="button"
                className={toolbarBtnClass(action.kind)}
                onClick={() => {
                  void action.onClick(helpers);
                }}
              >
                {formatFieldText(action.label)}
              </button>
            ))}
          </>
        );
      },
    [crudConfig, formatFieldText],
  );

  const filterRow = useCallback((row: StockLevelRow, q: string) => {
    const hay = normalize(
      [row.product_name, row.sku, String(row.quantity), String(row.min_quantity), row.is_low_stock ? 'bajo' : 'normal'].join(
        ' ',
      ),
    );
    return hay.includes(normalize(q));
  }, []);

  const statsLine = useCallback((visible: number, total: number) => {
    if (visible === total) {
      return `${total} ${total === 1 ? 'producto en el inventario' : 'productos en el inventario'}`;
    }
    return `${visible} de ${total} productos en el inventario (filtrado)`;
  }, []);

  return (
    <>
      <CrudKanbanSurface<StockLevelRow>
        columns={COLUMN_ORDER}
        columnIdSet={COLUMN_IDS}
        getRowColumnId={getRowColumnId}
        fallbackColumnId="wo_intake"
        items={items}
        loading={stockQuery.isLoading}
        error={combinedError}
        onMoveCard={handleMoveCard}
        resolveDropColumnId={(overId, snapshot) =>
          resolveStockDropColumnId(overId, snapshot, manualColumnById)
        }
        filterRow={filterRow}
        isRowDraggable={() => true}
        isColumnDroppable={() => true}
        onCardOpen={(row) => setDetailProductId(row.product_id)}
        renderCard={({ row, onOpen, suppressOpenRef }) => (
          <StockKanbanCardBody row={row} onOpen={onOpen} suppressOpenRef={suppressOpenRef} />
        )}
        renderOverlayCard={(row) => <StockKanbanCardPreview row={row} />}
        title="Inventario"
        subtitle="Misma grilla que órdenes de trabajo. Podés arrastrar tarjetas entre columnas (vista local; se resetea al recargar datos)."
        searchPlaceholder="Buscar..."
        toolbarButtonRow={
          <>
            {renderExtraToolbar({
              items,
              reload,
              setError: (message) => {
                setToolbarError(message);
              },
            })}
            <button type="button" className="btn-sm btn-primary" onClick={() => void navigate('/modules/products/list')}>
              + Nuevo producto
            </button>
          </>
        }
        statsLine={statsLine}
        columnFooter={() => (
          <Link to="/modules/products/list" className="m-kanban__column-add" draggable={false}>
            <span className="m-kanban__column-add-icon" aria-hidden="true">
              +
            </span>
            Añadir producto
          </Link>
        )}
      />
      <p className="text-secondary text-sm" style={{ marginTop: 'var(--space-3)', padding: '0 var(--space-3)' }}>
        Preferencias de vistas: <Link to="/modules/stock/configure">Configurar inventario</Link>.
      </p>
      <StockLevelDetailModal
        productId={detailProductId}
        onClose={() => setDetailProductId(null)}
        onAfterSave={() => {
          void queryClient.invalidateQueries({ queryKey: ['stock', 'inventory-kanban'] });
        }}
      />
    </>
  );
}
