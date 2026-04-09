import { NavLink, useMatch } from 'react-router-dom';
import '../styles/viewModeSegmentedSwitch.css';
import '../pages/WorkOrdersModuleSection.css';

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
  const isBoardActive = useMatch(boardPath);
  const isListMatch = useMatch(listPath);
  const isEditMatch = useMatch(editPattern ?? `${listPath}/edit/:orderId`);
  const isListContext = isListMatch || isEditMatch;

  return (
    <div className="wo-mod-orders__header-lead">
      {description && <p>{description}</p>}
      <div className="m-seg-switch" role="group" aria-label={groupAriaLabel}>
        <NavLink to={boardPath} className="m-seg-switch__track" draggable={false}>
          <span className={`m-seg-switch__label${isBoardActive ? ' m-seg-switch__label--active' : ''}`}>
            {leftLabel}
          </span>
          <span className={`m-seg-switch__label${!isBoardActive && isListContext ? ' m-seg-switch__label--active' : ''}`}>
            {rightLabel}
          </span>
          <span
            className={`m-seg-switch__thumb${isBoardActive ? ' m-seg-switch__thumb--left' : ' m-seg-switch__thumb--right'}`}
          />
        </NavLink>
        {!isBoardActive ? (
          <NavLink to={boardPath} className="m-seg-switch__hit m-seg-switch__hit--left" aria-hidden="true" draggable={false} tabIndex={-1}>&nbsp;</NavLink>
        ) : null}
        {isBoardActive ? (
          <NavLink to={listPath} className="m-seg-switch__hit m-seg-switch__hit--right" aria-hidden="true" draggable={false} tabIndex={-1}>&nbsp;</NavLink>
        ) : null}
      </div>
    </div>
  );
}
