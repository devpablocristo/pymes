import type { DashboardLayoutItem, DashboardWidgetDefinition } from '../types';

type WidgetCatalogProps = {
  open: boolean;
  widgets: DashboardWidgetDefinition[];
  layoutItems: DashboardLayoutItem[];
  onAdd: (widget: DashboardWidgetDefinition) => void;
  onClose: () => void;
};

export function WidgetCatalog({ open, widgets, layoutItems, onAdd, onClose }: WidgetCatalogProps) {
  if (!open) {
    return null;
  }

  return (
    <div className="dashboard-catalog-overlay" role="presentation" onClick={onClose}>
      <aside className="dashboard-catalog" role="dialog" aria-modal="true" onClick={(event) => event.stopPropagation()}>
        <div className="card-header">
          <div>
            <h2>Catalogo de widgets</h2>
            <p className="text-secondary">Solo aparecen widgets habilitados para tu rol y este contexto.</p>
          </div>
          <button type="button" className="btn-secondary btn-sm" onClick={onClose}>
            Cerrar
          </button>
        </div>

        <div className="dashboard-catalog-grid">
          {widgets.map((widget) => {
            const existing = layoutItems.find((item) => item.widget_key === widget.widget_key);
            const stateLabel = !existing ? 'Disponible' : existing.visible ? 'Visible' : 'Oculto';
            return (
              <article key={widget.widget_key} className="dashboard-catalog-card">
                <div className="dashboard-catalog-meta">
                  <span className="badge badge-neutral">{widget.domain}</span>
                  <span className="badge badge-neutral">{stateLabel}</span>
                </div>
                <strong>{widget.title}</strong>
                <p>{widget.description}</p>
                <small>
                  {widget.kind} · {widget.default_size.w}x{widget.default_size.h}
                </small>
                <button type="button" className="btn-primary btn-sm" onClick={() => onAdd(widget)}>
                  {existing?.visible ? 'Agregar otra instancia' : existing ? 'Mostrar' : 'Agregar al panel'}
                </button>
              </article>
            );
          })}
        </div>
      </aside>
    </div>
  );
}
