import { NavLink, Outlet, useMatch } from 'react-router-dom';
import { PageLayout } from '../components/PageLayout';
import { useI18n } from '../lib/i18n';
import './WorkOrdersModuleSection.css';

export function WorkOrdersModuleSection() {
  const { t } = useI18n();
  const isBoardActive = useMatch('/modules/workOrders/board');

  return (
    <PageLayout className="wo-mod-orders" title={t('shell.workOrders.pageTitle')} lead={t('shell.workOrders.pageLead')}>
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
    </PageLayout>
  );
}
