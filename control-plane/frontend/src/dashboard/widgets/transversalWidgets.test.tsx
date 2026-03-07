import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { SalesSummaryWidget, UnknownWidget } from './transversalWidgets';
import type { DashboardWidgetRendererProps } from '../types';

vi.mock('../hooks/useWidgetData', () => ({
  useDashboardWidgetData: () => ({
    data: undefined,
    isLoading: false,
    error: new Error('network down'),
  }),
}));

const props: DashboardWidgetRendererProps = {
  context: 'home',
  item: {
    widget_key: 'sales.summary',
    instance_id: 'sales-1',
    x: 0,
    y: 0,
    w: 4,
    h: 2,
    visible: true,
    settings: {},
    pinned: false,
    order_hint: 0,
  },
  widget: {
    widget_key: 'sales.summary',
    title: 'Ventas',
    description: 'Resumen de ventas',
    domain: 'control-plane',
    kind: 'metric',
    default_size: { w: 4, h: 2 },
    min_w: 3,
    min_h: 2,
    max_w: 6,
    max_h: 3,
    supported_contexts: ['home'],
    allowed_roles: ['admin'],
    data_endpoint: '/v1/dashboard-data/sales-summary',
    status: 'active',
  },
};

describe('dashboard widgets', () => {
  it('renders an error state when widget data fails', () => {
    render(<SalesSummaryWidget {...props} />);

    expect(screen.getByText('No se pudo cargar el widget')).toBeInTheDocument();
    expect(screen.getByText('network down')).toBeInTheDocument();
  });

  it('renders a fallback for widgets without a registered component', () => {
    render(
      <UnknownWidget
        {...props}
        widget={{
          ...props.widget,
          widget_key: 'professionals.today-agenda',
          data_endpoint: '/v1/dashboard-data/professionals/today-agenda',
        }}
      />,
    );

    expect(screen.getByText('Widget sin renderer local')).toBeInTheDocument();
    expect(screen.getAllByText(/professionals.today-agenda/)).toHaveLength(2);
  });
});
