import type { ReactNode } from 'react';

export type PageLayoutProps = {
  /** Título principal (h1) */
  title: ReactNode;
  /** Párrafo lead bajo el título (opcional) */
  lead?: ReactNode;
  /** Botones / enlaces a la derecha; activa layout tipo split automáticamente */
  actions?: ReactNode;
  /**
   * Contenido opcional entre cabecera y el cuerpo (alertas globales, avisos).
   * Para errores inline por sección seguir usando el patrón habitual en el hijo.
   */
  banner?: ReactNode;
  /** Clases extra en el contenedor (p. ej. `dash`, `gcal` para CSS por página) */
  className?: string;
  children: ReactNode;
};

function renderLead(lead: ReactNode) {
  return typeof lead === 'string' || typeof lead === 'number'
    ? <p>{lead}</p>
    : <div className="text-page-lead">{lead}</div>;
}

/**
 * Layout estándar de página en consola: `page-stack` + cabecera alineada con el resto del producto.
 *
 * No sustituye al shell CRUD (`LazyConfiguredCrudPage`): esas pantallas siguen usando el template del módulo.
 * Las páginas custom deben preferir este componente en lugar de copiar `<div className="page-stack">` + `<header className="page-header">`.
 *
 * La búsqueda global del shell (`usePageSearch`) sigue registrándose en la página hija; este layout no la reemplaza.
 */
export function PageLayout({ title, lead, actions, banner, className, children }: PageLayoutProps) {
  const stackClass = ['page-stack', className].filter(Boolean).join(' ');
  const hasActions = Boolean(actions);

  if (hasActions) {
    return (
      <div className={stackClass}>
        <header className="page-header page-header--split">
          <div className="page-header__main">
            <h1>{title}</h1>
            {lead ? renderLead(lead) : null}
          </div>
          <div className="page-header__actions">{actions}</div>
        </header>
        {banner}
        {children}
      </div>
    );
  }

  return (
    <div className={stackClass}>
      <header className="page-header">
        <h1>{title}</h1>
        {lead ? renderLead(lead) : null}
      </header>
      {banner}
      {children}
    </div>
  );
}
