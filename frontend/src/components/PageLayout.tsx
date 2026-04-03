import type { ReactNode } from 'react';
import { CrudPageShell } from '@devpablocristo/core-browser/crud';
import { usePageSearchShellControl } from './PageSearch';

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

function isPrimitiveLead(lead: ReactNode) {
  return typeof lead === 'string' || typeof lead === 'number';
}

/**
 * Layout estándar de página en consola: wrapper fino sobre el shell canónico compartido.
 *
 * Custom pages y CRUD deben salir del mismo origen visual/estructural. Por eso este componente
 * delega la cabecera a `CrudPageShell` en lugar de renderizar un header alternativo local.
 */
export function PageLayout({ title, lead, actions, banner, className, children }: PageLayoutProps) {
  const stackClass = ['page-stack', className].filter(Boolean).join(' ');
  const pageSearch = usePageSearchShellControl();
  const hasSearch = pageSearch.visible;
  const primitiveLead = lead != null && lead !== false && isPrimitiveLead(lead) ? lead : undefined;
  const richLead =
    lead != null && lead !== false && !isPrimitiveLead(lead) ? <div className="text-page-lead">{lead}</div> : undefined;
  return (
    <div className={stackClass}>
      <CrudPageShell
        title={title}
        subtitle={primitiveLead}
        headerLeadSlot={richLead}
        search={
          hasSearch
            ? {
                value: pageSearch.query,
                onChange: pageSearch.setQuery,
                placeholder: pageSearch.placeholder,
                clearLabel: 'Limpiar búsqueda',
              }
            : undefined
        }
        headerActions={actions}
      >
        <>
          {banner}
          {children}
        </>
      </CrudPageShell>
    </div>
  );
}
