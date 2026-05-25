export const HOME_DASHBOARD_CONTEXT = 'home';

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

export type TopService = {
  service_id: string;
  name: string;
  quantity: number;
  total: number;
};

export type TopServicesData = {
  period: string;
  items: TopService[];
};

export type TopCustomer = {
  customer_id?: string;
  party_id?: string;
  name: string;
  visit_count: number;
  total: number;
};

export type TopCustomersData = {
  period: string;
  items: TopCustomer[];
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
