import { NavLink, useMatch } from 'react-router-dom';
import '../pages/WorkOrdersModuleSection.css';

type Props = {
  boardPath: string;
  listPath: string;
  editPattern?: string;
  description?: string;
};

/**
 * Switch Board/Lista genérico para OT de cualquier vertical.
 */
export function WorkOrdersHeaderLead({ boardPath, listPath, editPattern, description }: Props) {
  const isBoardActive = useMatch(boardPath);
  const isListMatch = useMatch(listPath);
  const isEditMatch = useMatch(editPattern ?? `${listPath}/edit/:orderId`);
  const isListContext = isListMatch || isEditMatch;

  return (
    <div className="wo-mod-orders__header-lead">
      {description && <p>{description}</p>}
      <div className="wo-mod-orders__switch" role="group" aria-label="Navegación tablero / lista">
        <NavLink to={boardPath} className="wo-mod-orders__switch-track" draggable={false}>
          <span className={`wo-mod-orders__switch-label${isBoardActive ? ' wo-mod-orders__switch-label--active' : ''}`}>
            Tablero
          </span>
          <span className={`wo-mod-orders__switch-label${!isBoardActive && isListContext ? ' wo-mod-orders__switch-label--active' : ''}`}>
            Lista
          </span>
          <span className={`wo-mod-orders__switch-thumb${isBoardActive ? ' wo-mod-orders__switch-thumb--board' : ' wo-mod-orders__switch-thumb--list'}`} />
        </NavLink>
        {!isBoardActive ? (
          <NavLink to={boardPath} className="wo-mod-orders__switch-hit wo-mod-orders__switch-hit--left" aria-hidden="true" draggable={false} tabIndex={-1}>&nbsp;</NavLink>
        ) : null}
        {isBoardActive ? (
          <NavLink to={listPath} className="wo-mod-orders__switch-hit wo-mod-orders__switch-hit--right" aria-hidden="true" draggable={false} tabIndex={-1}>&nbsp;</NavLink>
        ) : null}
      </div>
    </div>
  );
}
