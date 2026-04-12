import type { KanbanColumnDef, SuppressCardOpen } from '@devpablocristo/modules-kanban-board';
import { useCallback, useEffect, useMemo, useState, type RefObject, type ReactNode } from 'react';
import type { CrudValueFilterOption } from '../../components/CrudPage';
import { CrudKanbanSurface } from './CrudKanbanSurface';
import './CrudValueKanbanSurface.css';

type ValueColumn<T extends { id: string }> = {
  id: string;
  label: string;
  matches: (row: T) => boolean;
};

type Props<T extends { id: string }> = {
  items: T[];
  loading: boolean;
  title: string;
  emptyLabel: string;
  valueFilterOptions?: CrudValueFilterOption<T>[];
  onCardOpen: (row: T) => void;
  getCardTitle: (row: T) => string;
  getCardSubtitle?: (row: T) => string;
  getCardMeta?: (row: T) => string;
  disableDrag?: boolean;
  columnFooter?: (columnId: string) => ReactNode;
};

function prettifyLabel(value: string) {
  return value
    .replace(/[_-]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/^\w/, (char) => char.toUpperCase());
}

function inferColumnsFromRows<T extends { id: string }>(items: T[]): ValueColumn<T>[] {
  const records = items as Array<Record<string, unknown>>;
  const inferField = () => {
    if (records.some((row) => typeof row.status === 'string')) return 'status';
    if (records.some((row) => typeof row.payment_status === 'string')) return 'payment_status';
    if (records.some((row) => typeof row.type === 'string')) return 'type';
    if (records.some((row) => typeof row.is_active === 'boolean')) return 'is_active';
    return null;
  };

  const field = inferField();
  if (!field) return [{ id: 'all', label: 'Todos', matches: () => true }];

  const values = Array.from(
    new Set(
      records
        .map((row) => {
          const raw = row[field];
          if (typeof raw === 'boolean') return raw ? 'active' : 'inactive';
          return String(raw ?? '').trim().toLowerCase();
        })
        .filter(Boolean),
    ),
  );

  if (values.length === 0) return [{ id: 'all', label: 'Todos', matches: () => true }];

  return values.map((value) => ({
    id: value,
    label: prettifyLabel(value),
    matches: (row: T) => {
      const raw = (row as Record<string, unknown>)[field];
      const normalized = typeof raw === 'boolean' ? (raw ? 'active' : 'inactive') : String(raw ?? '').trim().toLowerCase();
      return normalized === value;
    },
  }));
}

function resolveDropColumnId<T extends { id: string }>(
  overId: string | undefined,
  items: T[],
  getColumnId: (row: T) => string,
  columnIds: Set<string>,
): string | null {
  if (!overId) return null;
  if (overId.startsWith('col-')) {
    const columnId = overId.slice(4);
    return columnIds.has(columnId) ? columnId : null;
  }
  const overCard = items.find((item) => item.id === overId);
  if (!overCard) return null;
  const columnId = getColumnId(overCard);
  return columnIds.has(columnId) ? columnId : null;
}

function CrudValueKanbanCard<T extends { id: string }>({
  row,
  onOpen,
  suppressOpenRef,
  getCardTitle,
  getCardSubtitle,
  getCardMeta,
}: {
  row: T;
  onOpen: () => void;
  suppressOpenRef: RefObject<SuppressCardOpen>;
  getCardTitle: (row: T) => string;
  getCardSubtitle?: (row: T) => string;
  getCardMeta?: (row: T) => string;
}) {
  const handleClick = () => {
    const suppress = suppressOpenRef.current;
    if (suppress != null && suppress.id === row.id && Date.now() < suppress.until) return;
    onOpen();
  };

  const subtitle = getCardSubtitle?.(row) ?? '';
  const meta = getCardMeta?.(row) ?? '';

  return (
    <div
      className="m-kanban__card"
      onClick={handleClick}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          handleClick();
        }
      }}
      role="button"
      tabIndex={0}
      draggable={false}
    >
      <strong>{getCardTitle(row)}</strong>
      {subtitle ? <div className="m-kanban__card-meta">{subtitle}</div> : null}
      {meta ? <div className="m-kanban__card-meta">{meta}</div> : null}
    </div>
  );
}

function CrudValueKanbanOverlayCard<T extends { id: string }>({
  row,
  getCardTitle,
  getCardSubtitle,
  getCardMeta,
}: {
  row: T;
  getCardTitle: (row: T) => string;
  getCardSubtitle?: (row: T) => string;
  getCardMeta?: (row: T) => string;
}) {
  const subtitle = getCardSubtitle?.(row) ?? '';
  const meta = getCardMeta?.(row) ?? '';
  return (
    <div className="m-kanban__card m-kanban__card--overlay" aria-hidden="true">
      <strong>{getCardTitle(row)}</strong>
      {subtitle ? <div className="m-kanban__card-meta">{subtitle}</div> : null}
      {meta ? <div className="m-kanban__card-meta">{meta}</div> : null}
    </div>
  );
}

export function CrudValueKanbanSurface<T extends { id: string }>({
  items,
  loading,
  title,
  emptyLabel,
  valueFilterOptions = [],
  onCardOpen,
  getCardTitle,
  getCardSubtitle,
  getCardMeta,
  disableDrag = false,
  columnFooter,
}: Props<T>) {
  const [manualColumnById, setManualColumnById] = useState<Record<string, string>>({});

  useEffect(() => {
    setManualColumnById({});
  }, [items]);

  const columns = useMemo<ValueColumn<T>[]>(
    () =>
      valueFilterOptions.length > 0
        ? valueFilterOptions.map((option) => ({
            id: option.value,
            label: option.label,
            matches: option.matches,
          }))
        : inferColumnsFromRows(items),
    [items, valueFilterOptions],
  );

  const kanbanColumns = useMemo<KanbanColumnDef[]>(() => columns.map((column) => ({ id: column.id, label: column.label })), [columns]);
  const columnIdSet = useMemo(() => new Set(kanbanColumns.map((column) => column.id)), [kanbanColumns]);
  const fallbackColumnId = kanbanColumns[0]?.id ?? 'all';

  const getRowColumnId = useCallback(
    (row: T) => {
      const manual = manualColumnById[row.id];
      if (manual && columnIdSet.has(manual)) return manual;
      const resolved = columns.find((column) => column.matches(row))?.id;
      return resolved && columnIdSet.has(resolved) ? resolved : fallbackColumnId;
    },
    [columnIdSet, columns, fallbackColumnId, manualColumnById],
  );

  const handleMoveCard = useCallback(
    (id: string, targetColumnId: string) => {
      if (disableDrag || !columnIdSet.has(targetColumnId)) return;
      setManualColumnById((current) => ({ ...current, [id]: targetColumnId }));
    },
    [columnIdSet, disableDrag],
  );

  return (
    <div className="crud-value-kanban__board-only">
      <CrudKanbanSurface<T>
        columns={kanbanColumns}
        columnIdSet={columnIdSet}
        getRowColumnId={getRowColumnId}
        fallbackColumnId={fallbackColumnId}
        items={items}
        loading={loading}
        error={null}
        onMoveCard={handleMoveCard}
        resolveDropColumnId={(overId, snapshot) => resolveDropColumnId(overId, snapshot, getRowColumnId, columnIdSet)}
        renderCard={({ row, onOpen, suppressOpenRef }) => (
          <CrudValueKanbanCard
            row={row}
            onOpen={onOpen}
            suppressOpenRef={suppressOpenRef}
            getCardTitle={getCardTitle}
            getCardSubtitle={getCardSubtitle}
            getCardMeta={getCardMeta}
          />
        )}
        renderOverlayCard={(row) => (
          <CrudValueKanbanOverlayCard
            row={row}
            getCardTitle={getCardTitle}
            getCardSubtitle={getCardSubtitle}
            getCardMeta={getCardMeta}
          />
        )}
        onCardOpen={onCardOpen}
        title={title}
        externalSearch=""
        statsLine={() => ''}
        emptyState={<div className="empty-state"><p>{emptyLabel}</p></div>}
        isRowDraggable={() => !disableDrag}
        isColumnDroppable={() => !disableDrag}
        columnFooter={columnFooter}
      />
    </div>
  );
}
