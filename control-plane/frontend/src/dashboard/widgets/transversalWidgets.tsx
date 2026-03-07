import type { ReactNode } from 'react';
import { useDashboardWidgetData } from '../hooks/useWidgetData';
import type {
  AuditActivityData,
  BillingStatusData,
  CashflowSummaryData,
  DashboardWidgetRendererProps,
  LowStockData,
  QuotesPipelineData,
  RecentSalesData,
  SalesSummaryData,
  TopProductsData,
} from '../types';

export function SalesSummaryWidget(props: DashboardWidgetRendererProps) {
  const query = useDashboardWidgetData<SalesSummaryData>(props.widget, props.context);
  return (
    <WidgetQueryState query={query}>
      {(data) => (
        <div className="widget-metric-grid widget-metric-grid-3">
          <MetricTile label="Periodo" value={data.period} subtle />
          <MetricTile label="Ventas" value={formatAmount(data.total_sales)} />
          <MetricTile label="Operaciones" value={data.count_sales.toLocaleString('es-AR')} />
          <MetricTile label="Ticket promedio" value={formatAmount(data.average_ticket)} />
        </div>
      )}
    </WidgetQueryState>
  );
}

export function CashflowSummaryWidget(props: DashboardWidgetRendererProps) {
  const query = useDashboardWidgetData<CashflowSummaryData>(props.widget, props.context);
  return (
    <WidgetQueryState query={query}>
      {(data) => (
        <div className="widget-metric-grid widget-metric-grid-3">
          <MetricTile label="Periodo" value={data.period} subtle />
          <MetricTile label="Ingresos" value={formatAmount(data.total_income)} tone="success" />
          <MetricTile label="Egresos" value={formatAmount(data.total_expense)} tone="danger" />
          <MetricTile label="Balance" value={formatAmount(data.balance)} tone={data.balance >= 0 ? 'success' : 'danger'} />
        </div>
      )}
    </WidgetQueryState>
  );
}

export function QuotesPipelineWidget(props: DashboardWidgetRendererProps) {
  const query = useDashboardWidgetData<QuotesPipelineData>(props.widget, props.context);
  return (
    <WidgetQueryState query={query}>
      {(data) => (
        <div className="widget-metric-grid widget-metric-grid-4">
          <MetricTile label="Pendientes" value={data.pending_total.toLocaleString('es-AR')} />
          <MetricTile label="Draft" value={data.draft.toLocaleString('es-AR')} subtle />
          <MetricTile label="Enviados" value={data.sent.toLocaleString('es-AR')} subtle />
          <MetricTile label="Aceptados" value={data.accepted.toLocaleString('es-AR')} tone="success" />
          <MetricTile label="Rechazados" value={data.rejected.toLocaleString('es-AR')} tone="danger" />
        </div>
      )}
    </WidgetQueryState>
  );
}

export function LowStockWidget(props: DashboardWidgetRendererProps) {
  const query = useDashboardWidgetData<LowStockData>(props.widget, props.context);
  return (
    <WidgetQueryState query={query}>
      {(data) => (
        <>
          <div className="widget-stack-header">
            <strong>{data.total.toLocaleString('es-AR')} alertas activas</strong>
            <small>Se muestran las mas urgentes para operacion diaria.</small>
          </div>
          <div className="widget-list">
            {data.items.map((item) => (
              <div key={`${item.product_id}-${item.sku ?? ''}`} className="widget-list-row">
                <div>
                  <strong>{item.product_name || 'Producto sin nombre'}</strong>
                  <small>{item.sku || item.product_id}</small>
                </div>
                <div className="widget-row-metrics">
                  <span>{item.quantity.toLocaleString('es-AR')}</span>
                  <small>min {item.min_quantity.toLocaleString('es-AR')}</small>
                </div>
              </div>
            ))}
          </div>
        </>
      )}
    </WidgetQueryState>
  );
}

export function RecentSalesWidget(props: DashboardWidgetRendererProps) {
  const query = useDashboardWidgetData<RecentSalesData>(props.widget, props.context);
  return (
    <WidgetQueryState query={query}>
      {(data) => (
        <div className="widget-list">
          {data.items.map((item) => (
            <div key={item.id} className="widget-list-row">
              <div>
                <strong>{item.number}</strong>
                <small>{item.customer_name || 'Consumidor final'}</small>
              </div>
              <div className="widget-row-metrics">
                <span>
                  {item.currency} {item.total.toLocaleString('es-AR', { maximumFractionDigits: 2 })}
                </span>
                <small>{new Date(item.created_at).toLocaleString('es-AR')}</small>
              </div>
            </div>
          ))}
        </div>
      )}
    </WidgetQueryState>
  );
}

export function TopProductsWidget(props: DashboardWidgetRendererProps) {
  const query = useDashboardWidgetData<TopProductsData>(props.widget, props.context);
  return (
    <WidgetQueryState query={query}>
      {(data) => {
        const peak = Math.max(...data.items.map((item) => item.total), 1);
        return (
          <div className="widget-bars">
            {data.items.map((item) => (
              <div key={`${item.product_id}-${item.name}`} className="widget-bar-row">
                <div className="widget-bar-labels">
                  <strong>{item.name}</strong>
                  <small>
                    {item.quantity.toLocaleString('es-AR')} uds · {formatAmount(item.total)}
                  </small>
                </div>
                <div className="widget-bar-track">
                  <span style={{ width: `${Math.max((item.total / peak) * 100, 8)}%` }} />
                </div>
              </div>
            ))}
          </div>
        );
      }}
    </WidgetQueryState>
  );
}

export function BillingStatusWidget(props: DashboardWidgetRendererProps) {
  const query = useDashboardWidgetData<BillingStatusData>(props.widget, props.context);
  return (
    <WidgetQueryState query={query}>
      {(data) => (
        <div className="widget-stack">
          <div className="widget-status-card">
            <strong className="text-capitalize">Plan {data.plan_code}</strong>
            <span className={`badge ${billingBadgeClass(data.status)}`}>{data.status}</span>
          </div>
          <div className="widget-pill-grid">
            {Object.entries(data.hard_limits ?? {}).map(([key, value]) => (
              <div key={key} className="widget-pill-card">
                <small>{key.replace(/_/g, ' ')}</small>
                <strong>{String(value)}</strong>
              </div>
            ))}
          </div>
          {data.updated_at ? <small>Actualizado {new Date(data.updated_at).toLocaleString('es-AR')}</small> : null}
        </div>
      )}
    </WidgetQueryState>
  );
}

export function AuditActivityWidget(props: DashboardWidgetRendererProps) {
  const query = useDashboardWidgetData<AuditActivityData>(props.widget, props.context);
  return (
    <WidgetQueryState query={query}>
      {(data) => (
        <div className="widget-timeline">
          {data.items.map((item) => (
            <div key={item.id} className="widget-timeline-row">
              <span className="widget-timeline-dot" />
              <div>
                <strong>{item.action}</strong>
                <small>
                  {item.actor || 'system'} · {item.resource_type}
                  {item.resource_id ? ` · ${item.resource_id}` : ''}
                </small>
              </div>
              <time>{new Date(item.created_at).toLocaleString('es-AR')}</time>
            </div>
          ))}
        </div>
      )}
    </WidgetQueryState>
  );
}

export function UnknownWidget(props: DashboardWidgetRendererProps) {
  return (
    <div className="widget-state widget-state-warning">
      <strong>Widget sin renderer local</strong>
      <p>
        `{props.widget.widget_key}` existe en el catalogo, pero este frontend no tiene un componente registrado todavia.
      </p>
      <small>{props.widget.data_endpoint}</small>
    </div>
  );
}

type QueryLike<T> = {
  data: T | undefined;
  isLoading: boolean;
  error: Error | null;
};

function WidgetQueryState<T>({
  query,
  children,
}: {
  query: QueryLike<T>;
  children: (data: T) => ReactNode;
}) {
  if (query.isLoading) {
    return <div className="widget-state widget-state-loading">Cargando widget...</div>;
  }
  if (query.error) {
    return (
      <div className="widget-state widget-state-error">
        <strong>No se pudo cargar el widget</strong>
        <p>{query.error.message}</p>
      </div>
    );
  }
  if (!query.data) {
    return <div className="widget-state widget-state-warning">El widget no devolvio datos.</div>;
  }
  return <>{children(query.data)}</>;
}

function MetricTile({
  label,
  value,
  subtle,
  tone,
}: {
  label: string;
  value: string;
  subtle?: boolean;
  tone?: 'success' | 'danger';
}) {
  return (
    <div className={`widget-metric-tile${subtle ? ' subtle' : ''}${tone ? ` ${tone}` : ''}`}>
      <small>{label}</small>
      <strong>{value}</strong>
    </div>
  );
}

function formatAmount(value: number): string {
  return `$ ${value.toLocaleString('es-AR', {
    maximumFractionDigits: 2,
    minimumFractionDigits: value % 1 === 0 ? 0 : 2,
  })}`;
}

function billingBadgeClass(status: string): string {
  switch (status) {
    case 'active':
    case 'trialing':
      return 'badge-success';
    case 'past_due':
      return 'badge-warning';
    default:
      return 'badge-danger';
  }
}
