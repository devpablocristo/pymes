import type { ComponentType } from 'react';
import type { DashboardWidgetRendererProps } from '../types';
import {
  AuditActivityWidget,
  BillingStatusWidget,
  CashflowSummaryWidget,
  LowStockWidget,
  QuotesPipelineWidget,
  RecentSalesWidget,
  SalesSummaryWidget,
  TopProductsWidget,
  UnknownWidget,
} from '../widgets/transversalWidgets';

const dashboardWidgetRegistry: Record<string, ComponentType<DashboardWidgetRendererProps>> = {
  'sales.summary': SalesSummaryWidget,
  'cashflow.summary': CashflowSummaryWidget,
  'quotes.pipeline': QuotesPipelineWidget,
  'inventory.low_stock': LowStockWidget,
  'sales.recent': RecentSalesWidget,
  'products.top': TopProductsWidget,
  'billing.subscription': BillingStatusWidget,
  'audit.activity': AuditActivityWidget,
};

export function resolveDashboardWidget(widgetKey: string): ComponentType<DashboardWidgetRendererProps> {
  return dashboardWidgetRegistry[widgetKey] ?? UnknownWidget;
}

export { dashboardWidgetRegistry };
