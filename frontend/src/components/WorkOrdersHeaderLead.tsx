import { CrudViewModeSwitch } from '../modules/crud';

type Props = {
  boardPath: string;
  listPath: string;
  /** Primer segmento del switch (p. ej. Tablero, Galería). */
  leftLabel?: string;
  /** Segundo segmento (p. ej. Lista, Tabla). */
  rightLabel?: string;
  /** aria-label del grupo; por defecto copy para OT. */
  groupAriaLabel?: string;
  editPattern?: string;
  description?: string;
};

/**
 * Switch de dos vistas en rutas hermanas (Tablero/Lista en OT, Galería/Lista en productos, etc.).
 */
export function WorkOrdersHeaderLead({
  boardPath,
  listPath,
  leftLabel = 'Tablero',
  rightLabel = 'Lista',
  groupAriaLabel = 'Navegación tablero / lista',
  editPattern,
  description,
}: Props) {
  return (
    <CrudViewModeSwitch
      primaryPath={boardPath}
      secondaryPath={listPath}
      primaryLabel={leftLabel}
      secondaryLabel={rightLabel}
      groupAriaLabel={groupAriaLabel}
      secondaryContextPattern={editPattern ?? `${listPath}/edit/:orderId`}
      description={description}
    />
  );
}
