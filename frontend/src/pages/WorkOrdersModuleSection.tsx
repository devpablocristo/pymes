import { NavLink, Outlet } from 'react-router-dom';
import { useI18n } from '../lib/i18n';
import './WorkOrdersModuleSection.css';

/**
 * Módulo órdenes de trabajo: selector Tablero / Lista bajo /modules/workOrders.
 */
export function WorkOrdersModuleSection() {
  const { t } = useI18n();

  return (
    <div className="wo-mod-orders">
      <div
        className="wo-mod-orders__segmented"
        role="group"
        aria-label={t('shell.workOrders.tabsNavAria')}
      >
        <NavLink
          to="board"
          className={({ isActive }) =>
            `wo-mod-orders__segment${isActive ? ' wo-mod-orders__segment--active' : ''}`
          }
        >
          {t('shell.workOrders.tabBoard')}
        </NavLink>
        <span className="wo-mod-orders__slash" aria-hidden="true">
          /
        </span>
        <NavLink
          to="list"
          className={({ isActive }) =>
            `wo-mod-orders__segment${isActive ? ' wo-mod-orders__segment--active' : ''}`
          }
        >
          {t('shell.workOrders.tabList')}
        </NavLink>
      </div>
      <Outlet />
    </div>
  );
}
