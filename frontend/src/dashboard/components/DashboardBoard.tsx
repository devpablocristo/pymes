import { resolveDashboardWidget } from '../registry';
import type { DashboardContext, DashboardLayoutItem, DashboardWidgetDefinition } from '../types';
import { visibleItems } from '../utils/layout';
import { WidgetFrame } from './WidgetFrame';

type DashboardBoardProps = {
  context: DashboardContext;
  items: DashboardLayoutItem[];
  widgets: DashboardWidgetDefinition[];
  editing: boolean;
  onMoveBackward: (instanceId: string) => void;
  onMoveForward: (instanceId: string) => void;
  onGrow: (instanceId: string) => void;
  onShrink: (instanceId: string) => void;
  onToggleVisibility: (instanceId: string) => void;
};

export function DashboardBoard({
  context,
  items,
  widgets,
  editing,
  onMoveBackward,
  onMoveForward,
  onGrow,
  onShrink,
  onToggleVisibility,
}: DashboardBoardProps) {
  const widgetsMap = widgets.reduce<Record<string, DashboardWidgetDefinition>>((acc, widget) => {
    acc[widget.widget_key] = widget;
    return acc;
  }, {});
  const renderedItems = visibleItems(items);

  if (renderedItems.length === 0) {
    return (
      <div className="dashboard-empty card">
        <h2>No hay widgets visibles</h2>
        <p>Activa widgets desde el catalogo o resetea este contexto para volver al layout base.</p>
      </div>
    );
  }

  return (
    <section className="dashboard-grid" aria-label="Dashboard widgets">
      {renderedItems.map((item) => {
        const widget = widgetsMap[item.widget_key];
        if (!widget) {
          return null;
        }
        const Renderer = resolveDashboardWidget(widget.widget_key);
        return (
          <WidgetFrame
            key={item.instance_id}
            widget={widget}
            item={item}
            editing={editing}
            onMoveBackward={onMoveBackward}
            onMoveForward={onMoveForward}
            onGrow={onGrow}
            onShrink={onShrink}
            onToggleVisibility={onToggleVisibility}
          >
            <Renderer context={context} item={item} widget={widget} />
          </WidgetFrame>
        );
      })}
    </section>
  );
}
