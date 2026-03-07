import type { DashboardLayoutItem, DashboardSavePayload, DashboardWidgetDefinition } from '../types';

export const GRID_COLUMNS = 12;

export function indexWidgets(widgets: DashboardWidgetDefinition[]): Record<string, DashboardWidgetDefinition> {
  return widgets.reduce<Record<string, DashboardWidgetDefinition>>((acc, widget) => {
    acc[widget.widget_key] = widget;
    return acc;
  }, {});
}

export function sortLayoutItems(items: DashboardLayoutItem[]): DashboardLayoutItem[] {
  return [...items].sort((left, right) => {
    if (left.pinned !== right.pinned) {
      return left.pinned ? -1 : 1;
    }
    if (left.order_hint !== right.order_hint) {
      return left.order_hint - right.order_hint;
    }
    if (left.y !== right.y) {
      return left.y - right.y;
    }
    if (left.x !== right.x) {
      return left.x - right.x;
    }
    return left.instance_id.localeCompare(right.instance_id);
  });
}

export function packLayoutItems(
  items: DashboardLayoutItem[],
  widgets: DashboardWidgetDefinition[],
): DashboardLayoutItem[] {
  const widgetMap = indexWidgets(widgets);
  const normalized = sortLayoutItems(items)
    .filter((item) => widgetMap[item.widget_key])
    .map((item) => clampLayoutItem(item, widgetMap[item.widget_key]));

  let cursorX = 0;
  let cursorY = 0;
  let rowHeight = 0;

  return normalized.map((item, index) => {
    if (!item.visible) {
      return { ...item, x: 0, y: 0, order_hint: index };
    }
    if (cursorX + item.w > GRID_COLUMNS) {
      cursorY += Math.max(rowHeight, 1);
      cursorX = 0;
      rowHeight = 0;
    }
    const packed = {
      ...item,
      x: cursorX,
      y: cursorY,
      order_hint: index,
    };
    cursorX += item.w;
    rowHeight = Math.max(rowHeight, item.h);
    return packed;
  });
}

export function moveLayoutItem(
  items: DashboardLayoutItem[],
  instanceId: string,
  delta: -1 | 1,
  widgets: DashboardWidgetDefinition[],
): DashboardLayoutItem[] {
  const ordered = sortLayoutItems(items);
  const index = ordered.findIndex((item) => item.instance_id === instanceId);
  if (index === -1) {
    return ordered;
  }
  const target = index + delta;
  if (target < 0 || target >= ordered.length) {
    return ordered;
  }
  const next = [...ordered];
  [next[index], next[target]] = [next[target], next[index]];
  next.forEach((item, itemIndex) => {
    item.order_hint = itemIndex;
  });
  return packLayoutItems(next, widgets);
}

export function resizeLayoutItem(
  items: DashboardLayoutItem[],
  instanceId: string,
  deltaW: number,
  deltaH: number,
  widgets: DashboardWidgetDefinition[],
): DashboardLayoutItem[] {
  const widgetMap = indexWidgets(widgets);
  const next = items.map((item) => {
    if (item.instance_id !== instanceId) {
      return item;
    }
    const widget = widgetMap[item.widget_key];
    if (!widget) {
      return item;
    }
    return {
      ...item,
      w: clampNumber(item.w + deltaW, widget.min_w, widget.max_w),
      h: clampNumber(item.h + deltaH, widget.min_h, widget.max_h),
    };
  });
  return packLayoutItems(next, widgets);
}

export function toggleLayoutItemVisibility(
  items: DashboardLayoutItem[],
  instanceId: string,
  widgets: DashboardWidgetDefinition[],
): DashboardLayoutItem[] {
  return packLayoutItems(
    items.map((item) =>
      item.instance_id === instanceId ? { ...item, visible: !item.visible } : item,
    ),
    widgets,
  );
}

export function upsertWidgetInstance(
  items: DashboardLayoutItem[],
  widget: DashboardWidgetDefinition,
  widgets: DashboardWidgetDefinition[],
): DashboardLayoutItem[] {
  const existingHidden = items.find(
    (item) => item.widget_key === widget.widget_key && !item.visible,
  );
  if (existingHidden) {
    return packLayoutItems(
      items.map((item) =>
        item.instance_id === existingHidden.instance_id ? { ...item, visible: true } : item,
      ),
      widgets,
    );
  }
  const next = [
    ...items,
    {
      widget_key: widget.widget_key,
      instance_id: createWidgetInstanceId(widget.widget_key),
      x: 0,
      y: 0,
      w: widget.default_size.w,
      h: widget.default_size.h,
      visible: true,
      settings: {},
      pinned: false,
      order_hint: items.length,
    },
  ];
  return packLayoutItems(next, widgets);
}

export function serializeLayout(
  items: DashboardLayoutItem[],
  widgets: DashboardWidgetDefinition[],
): string {
  return JSON.stringify(packLayoutItems(items, widgets));
}

export function toDashboardSavePayload(
  context: string,
  items: DashboardLayoutItem[],
  widgets: DashboardWidgetDefinition[],
): DashboardSavePayload {
  return {
    context,
    items: packLayoutItems(items, widgets),
  };
}

export function visibleItems(items: DashboardLayoutItem[]): DashboardLayoutItem[] {
  return sortLayoutItems(items).filter((item) => item.visible);
}

export function hiddenItems(items: DashboardLayoutItem[]): DashboardLayoutItem[] {
  return sortLayoutItems(items).filter((item) => !item.visible);
}

function clampLayoutItem(
  item: DashboardLayoutItem,
  widget: DashboardWidgetDefinition,
): DashboardLayoutItem {
  return {
    ...item,
    w: clampNumber(item.w || widget.default_size.w, widget.min_w, widget.max_w),
    h: clampNumber(item.h || widget.default_size.h, widget.min_h, widget.max_h),
    settings: item.settings ?? {},
  };
}

function clampNumber(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

function createWidgetInstanceId(widgetKey: string): string {
  const suffix =
    globalThis.crypto?.randomUUID?.().slice(0, 8) ?? Math.random().toString(16).slice(2, 10);
  return `${widgetKey.replace(/[^a-z0-9]+/gi, '-')}-${suffix}`.toLowerCase();
}
