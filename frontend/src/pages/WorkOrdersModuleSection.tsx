import { NavLink, Outlet, useMatch } from 'react-router-dom';
import { useI18n } from '../lib/i18n';
import './WorkOrdersModuleSection.css';

export function WorkOrdersModuleSection() {
  const { t } = useI18n();
  const isBoardActive = useMatch('/modules/workOrders/board');

  return (
    <div className="wo-mod-orders page-stack">
      <header className="page-header">
        <h1>{t('shell.workOrders.pageTitle')}</h1>
        <p>{t('shell.workOrders.pageLead')}</p>
      </header>
      <div className="wo-mod-orders__switch" role="group" aria-label={t('shell.workOrders.tabsNavAria')}>
        <NavLink to="board" className="wo-mod-orders__switch-track" draggable={false}>
          <span className={`wo-mod-orders__switch-label${isBoardActive ? ' wo-mod-orders__switch-label--active' : ''}`}>
            {t('shell.workOrders.tabBoard')}
          </span>
          <span className={`wo-mod-orders__switch-label${!isBoardActive ? ' wo-mod-orders__switch-label--active' : ''}`}>
            {t('shell.workOrders.tabList')}
          </span>
          <span
            className={`wo-mod-orders__switch-thumb${isBoardActive ? ' wo-mod-orders__switch-thumb--board' : ' wo-mod-orders__switch-thumb--list'}`}
          />
        </NavLink>
        {!isBoardActive && <NavLink to="board" className="wo-mod-orders__switch-hit wo-mod-orders__switch-hit--left" aria-hidden="true" draggable={false} tabIndex={-1}>&nbsp;</NavLink>}
        {isBoardActive && <NavLink to="list" className="wo-mod-orders__switch-hit wo-mod-orders__switch-hit--right" aria-hidden="true" draggable={false} tabIndex={-1}>&nbsp;</NavLink>}
      </div>
      <Outlet />
    </div>
  );
}
