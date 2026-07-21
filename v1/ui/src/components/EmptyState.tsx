import type { ReactNode } from 'react';

type EmptyStateProps = {
  /** Título principal del estado vacío. */
  title?: string;
  /** Descripción complementaria del estado vacío. */
  description?: ReactNode;
  /** Ícono opcional (`<IconX size={32} />` o equivalente). */
  icon?: ReactNode;
  /** CTA opcional (button, link). */
  action?: ReactNode;
  /** Override del className base si se necesita layout específico. */
  className?: string;
};

/**
 * EmptyState — placeholder unificado para listas y vistas sin datos.
 *
 * Usa la clase `.empty-state` definida en `styles/components.css`
 * (centrado, padding generoso, color text-muted). Pensado para
 * consumirse desde CRUD lists, dashboards con data vacía, búsquedas
 * sin resultados, etc.
 *
 * @example
 *   <EmptyState
 *     title="Sin clientes todavía"
 *     description="Creá el primero o importá un CSV."
 *     action={<button className="btn-primary">Nuevo cliente</button>}
 *   />
 */
export function EmptyState({ title, description, icon, action, className }: EmptyStateProps) {
  return (
    <div className={className ?? 'empty-state'} role="status">
      {icon ? <div className="empty-state__icon">{icon}</div> : null}
      {title ? <h3 className="empty-state__title">{title}</h3> : null}
      {description ? <p>{description}</p> : null}
      {action ? <div className="empty-state__action">{action}</div> : null}
    </div>
  );
}
