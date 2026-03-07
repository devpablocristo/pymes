import { describe, expect, it } from 'vitest';
import type { DashboardLayoutItem, DashboardWidgetDefinition } from '../types';
import {
  moveLayoutItem,
  packLayoutItems,
  resizeLayoutItem,
  toggleLayoutItemVisibility,
  upsertWidgetInstance,
} from './layout';

const widgets: DashboardWidgetDefinition[] = [
  {
    widget_key: 'sales.summary',
    title: 'Ventas',
    description: 'Resumen',
    domain: 'control-plane',
    kind: 'metric',
    default_size: { w: 4, h: 2 },
    min_w: 3,
    min_h: 2,
    max_w: 6,
    max_h: 3,
    supported_contexts: ['home'],
    allowed_roles: ['admin', 'member'],
    data_endpoint: '/v1/dashboard-data/sales-summary',
    status: 'active',
  },
  {
    widget_key: 'audit.activity',
    title: 'Actividad',
    description: 'Feed',
    domain: 'control-plane',
    kind: 'feed',
    default_size: { w: 6, h: 3 },
    min_w: 4,
    min_h: 3,
    max_w: 8,
    max_h: 6,
    supported_contexts: ['home'],
    allowed_roles: ['admin', 'member'],
    data_endpoint: '/v1/dashboard-data/audit-activity',
    status: 'active',
  },
];

const baseItems: DashboardLayoutItem[] = [
  {
    widget_key: 'sales.summary',
    instance_id: 'sales-1',
    x: 0,
    y: 0,
    w: 4,
    h: 2,
    visible: true,
    settings: {},
    pinned: true,
    order_hint: 0,
  },
  {
    widget_key: 'audit.activity',
    instance_id: 'audit-1',
    x: 4,
    y: 0,
    w: 6,
    h: 3,
    visible: true,
    settings: {},
    pinned: false,
    order_hint: 1,
  },
];

describe('layout utils', () => {
  it('packs dashboard items into deterministic grid coordinates', () => {
    const packed = packLayoutItems(baseItems, widgets);

    expect(packed[0]).toMatchObject({ instance_id: 'sales-1', x: 0, y: 0, order_hint: 0 });
    expect(packed[1]).toMatchObject({ instance_id: 'audit-1', x: 4, y: 0, order_hint: 1 });
  });

  it('moves and resizes widgets while keeping layout valid', () => {
    const moved = moveLayoutItem(
      baseItems.map((item) => ({ ...item, pinned: false })),
      'audit-1',
      -1,
      widgets,
    );
    expect(moved[0].instance_id).toBe('audit-1');

    const resized = resizeLayoutItem(moved, 'audit-1', 1, 1, widgets);
    expect(resized[0].w).toBe(7);
    expect(resized[0].h).toBe(4);
  });

  it('hides and restores widgets from the catalog flow', () => {
    const hidden = toggleLayoutItemVisibility(baseItems, 'sales-1', widgets);
    expect(hidden.find((item) => item.instance_id === 'sales-1')?.visible).toBe(false);

    const restored = upsertWidgetInstance(hidden, widgets[0], widgets);
    expect(restored.find((item) => item.instance_id === 'sales-1')?.visible).toBe(true);
  });
});
