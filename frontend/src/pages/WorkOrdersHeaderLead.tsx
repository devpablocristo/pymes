import { NavLink, useMatch } from 'react-router-dom';
import { useI18n } from '../lib/i18n';

export function WorkOrdersHeaderLead() {
  const { t } = useI18n();
  const isBoardActive = useMatch('/modules/workOrders/board');
  const isListMatch = useMatch('/modules/workOrders/list');
  const isEditMatch = useMatch('/modules/workOrders/edit/:orderId');
  const isListContext = isListMatch || isEditMatch;

  return (
    <div className="wo-mod-orders__header-lead">
      <p>{t('shell.workOrders.pageLead')}</p>
      <div className="wo-mod-orders__switch" role="group" aria-label={t('shell.workOrders.tabsNavAria')}>
        <NavLink to="/modules/workOrders/board" className="wo-mod-orders__switch-track" draggable={false}>
          <span className={`wo-mod-orders__switch-label${isBoardActive ? ' wo-mod-orders__switch-label--active' : ''}`}>
            {t('shell.workOrders.tabBoard')}
          </span>
          <span
            className={`wo-mod-orders__switch-label${!isBoardActive && isListContext ? ' wo-mod-orders__switch-label--active' : ''}`}
          >
            {t('shell.workOrders.tabList')}
          </span>
          <span
            className={`wo-mod-orders__switch-thumb${isBoardActive ? ' wo-mod-orders__switch-thumb--board' : ' wo-mod-orders__switch-thumb--list'}`}
          />
        </NavLink>
        {!isBoardActive ? (
          <NavLink
            to="/modules/workOrders/board"
            className="wo-mod-orders__switch-hit wo-mod-orders__switch-hit--left"
            aria-hidden="true"
            draggable={false}
            tabIndex={-1}
          >
            &nbsp;
          </NavLink>
        ) : null}
        {isBoardActive ? (
          <NavLink
            to="/modules/workOrders/list"
            className="wo-mod-orders__switch-hit wo-mod-orders__switch-hit--right"
            aria-hidden="true"
            draggable={false}
            tabIndex={-1}
          >
            &nbsp;
          </NavLink>
        ) : null}
      </div>
    </div>
  );
}
