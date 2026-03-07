import type { PropsWithChildren } from 'react';
import type { DashboardLayoutItem, DashboardWidgetDefinition } from '../types';

type WidgetFrameProps = PropsWithChildren<{
  widget: DashboardWidgetDefinition;
  item: DashboardLayoutItem;
  editing: boolean;
  onMoveBackward: (instanceId: string) => void;
  onMoveForward: (instanceId: string) => void;
  onGrow: (instanceId: string) => void;
  onShrink: (instanceId: string) => void;
  onToggleVisibility: (instanceId: string) => void;
}>;

export function WidgetFrame({
  widget,
  item,
  editing,
  onMoveBackward,
  onMoveForward,
  onGrow,
  onShrink,
  onToggleVisibility,
  children,
}: WidgetFrameProps) {
  return (
    <article
      className={`dashboard-widget-card${editing ? ' editing' : ''}${item.visible ? '' : ' muted'}`}
      style={{
        gridColumn: `${item.x + 1} / span ${item.w}`,
        gridRow: `${item.y + 1} / span ${item.h}`,
      }}
    >
      <header className="dashboard-widget-header">
        <div>
          <div className="dashboard-widget-kicker">
            <span className="badge badge-neutral">{widget.domain}</span>
            <span className="badge badge-neutral">{widget.kind}</span>
            {item.pinned ? <span className="badge badge-warning">Pinned</span> : null}
          </div>
          <h3>{widget.title}</h3>
          <p>{widget.description}</p>
        </div>
        {editing ? (
          <div className="dashboard-widget-toolbar">
            <button type="button" className="btn-secondary btn-sm" onClick={() => onMoveBackward(item.instance_id)}>
              Subir
            </button>
            <button type="button" className="btn-secondary btn-sm" onClick={() => onMoveForward(item.instance_id)}>
              Bajar
            </button>
            <button type="button" className="btn-secondary btn-sm" onClick={() => onShrink(item.instance_id)}>
              Compactar
            </button>
            <button type="button" className="btn-secondary btn-sm" onClick={() => onGrow(item.instance_id)}>
              Expandir
            </button>
            <button type="button" className="btn-secondary btn-sm" onClick={() => onToggleVisibility(item.instance_id)}>
              {item.visible ? 'Ocultar' : 'Mostrar'}
            </button>
          </div>
        ) : null}
      </header>

      <div className="dashboard-widget-body">{children}</div>
    </article>
  );
}
