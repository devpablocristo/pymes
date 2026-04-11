import { NavLink, Outlet, useLocation } from 'react-router-dom';
import './StockPage.css';
import './StockModuleSection.css';

const TABS: Array<{ to: string; label: string; end?: boolean }> = [
  { to: '/modules/stock/list', label: 'Lista', end: true },
  { to: '/modules/stock/gallery', label: 'Galería' },
  { to: '/modules/stock/board', label: 'Tablero' },
];

export function StockModuleSection() {
  const { pathname } = useLocation();
  const onConfigurePage = pathname.includes('/modules/stock/configure');

  return (
    <div className="stock-mod">
      <div className="stock-mod__bar">
        <nav className="stock-mod__tabs" aria-label="Vistas de inventario">
          {TABS.map((tab) => (
            <NavLink
              key={tab.to}
              to={tab.to}
              end={tab.end === true}
              className={({ isActive }) => `stock-mod__tab${isActive ? ' stock-mod__tab--active' : ''}`}
            >
              {tab.label}
            </NavLink>
          ))}
        </nav>
        {!onConfigurePage ? (
          <NavLink className="stock-mod__settings" to="/modules/stock/configure">
            Configurar
          </NavLink>
        ) : null}
      </div>
      <Outlet />
    </div>
  );
}
