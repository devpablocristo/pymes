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
    <div className="dashboard-catalog-overlay app-modal-backdrop" role="presentation" onClick={onClose}>
      <aside
        className="dashboard-catalog app-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="dashboard-catalog-title"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="dashboard-catalog__header app-modal__header">
          <div className="app-modal__title-block">
            <h2 id="dashboard-catalog-title" className="app-modal__title">
              Catálogo de widgets
            </h2>
            <p className="app-modal__subtitle">Solo aparecen widgets habilitados para tu rol y este contexto.</p>
          </div>
          <button type="button" className="app-modal__close" onClick={onClose} aria-label="Cerrar">
            ×
          </button>
        </div>

        <div className="dashboard-catalog__body app-modal__body">
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
                  <button type="button" className="btn-primary btn-sm app-modal__action" onClick={() => onAdd(widget)}>
                    {existing?.visible ? 'Agregar otra instancia' : existing ? 'Mostrar' : 'Agregar al panel'}
                  </button>
                </article>
              );
            })}
          </div>
        </div>

        <div className="dashboard-catalog__footer app-modal__footer">
          <div className="app-modal__footer-spacer" aria-hidden />
          <button type="button" className="btn-secondary btn-sm app-modal__action" onClick={onClose}>
            Cerrar
          </button>
        </div>
      </aside>
    </div>
  );
}
