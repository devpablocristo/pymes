export type DashboardContext = 'home' | 'commercial' | 'operations' | 'control' | string;

export type DashboardWidgetSize = {
  w: number;
  h: number;
};

export type DashboardWidgetDefinition = {
  widget_key: string;
  title: string;
  description: string;
  domain: string;
  kind: string;
  default_size: DashboardWidgetSize;
  min_w: number;
  min_h: number;
  max_w: number;
  max_h: number;
  supported_contexts: string[];
  allowed_roles: string[];
  required_scopes?: string[];
  settings_schema?: Record<string, unknown>;
  data_endpoint: string;
  status: string;
};

export type DashboardLayoutItem = {
  widget_key: string;
  instance_id: string;
  x: number;
  y: number;
  w: number;
  h: number;
  visible: boolean;
  settings: Record<string, unknown>;
  pinned: boolean;
  order_hint: number;
};

export type DashboardLayout = {
  source: string;
  layout_key: string;
  version: number;
  items: DashboardLayoutItem[];
};

export type DashboardResponse = {
  context: DashboardContext;
  layout: DashboardLayout;
  available_widgets: DashboardWidgetDefinition[];
};

export type DashboardWidgetCatalogResponse = {
  context: DashboardContext;
  items: DashboardWidgetDefinition[];
};

export type DashboardSavePayload = {
  context: DashboardContext;
  items: DashboardLayoutItem[];
};

export type DashboardWidgetRendererProps = {
  context: DashboardContext;
  item: DashboardLayoutItem;
  widget: DashboardWidgetDefinition;
};

export type DashboardContextDefinition = {
  id: DashboardContext;
  label: string;
  kicker: string;
  description: string;
};

export const dashboardContexts: DashboardContextDefinition[] = [
  {
    id: 'home',
    label: 'Panel',
    kicker: 'Base estable',
    description: 'La vista inicial transversal para cada usuario autenticado.',
  },
  {
    id: 'commercial',
    label: 'Comercial',
    kicker: 'Embudo',
    description: 'Prioriza ventas, presupuestos y actividad del frente comercial.',
  },
  {
    id: 'operations',
    label: 'Operaciones',
    kicker: 'Ejecución',
    description: 'Expone alertas operativas, stock y ritmo diario del tenant.',
  },
  {
    id: 'control',
    label: 'Control',
    kicker: 'Gobierno',
    description: 'Concentra billing, cashflow, auditoria y supervision.',
  },
];

export type SalesSummaryData = {
  period: string;
  total_sales: number;
  count_sales: number;
  average_ticket: number;
};

export type CashflowSummaryData = {
  period: string;
  total_income: number;
  total_expense: number;
  balance: number;
};

export type QuotesPipelineData = {
  draft: number;
  sent: number;
  accepted: number;
  rejected: number;
  pending_total: number;
};

export type LowStockItem = {
  product_id: string;
  product_name: string;
  sku?: string;
  quantity: number;
  min_quantity: number;
};

export type LowStockData = {
  total: number;
  items: LowStockItem[];
};

export type RecentSale = {
  id: string;
  number: string;
  customer_name: string;
  total: number;
  currency: string;
  created_at: string;
};

export type RecentSalesData = {
  items: RecentSale[];
};

export type TopProduct = {
  product_id: string;
  name: string;
  quantity: number;
  total: number;
};

export type TopProductsData = {
  period: string;
  items: TopProduct[];
};

export type BillingStatusData = {
  plan_code: string;
  status: string;
  hard_limits: Record<string, unknown>;
  updated_at?: string;
};

export type AuditActivityItem = {
  id: string;
  actor: string;
  action: string;
  resource_type: string;
  resource_id?: string;
  created_at: string;
};

export type AuditActivityData = {
  items: AuditActivityItem[];
};
