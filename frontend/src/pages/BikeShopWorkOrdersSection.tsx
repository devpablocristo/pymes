import { Outlet } from 'react-router-dom';
import '../pages/WorkOrdersModuleSection.css';

export function BikeShopWorkOrdersSection() {
  return (
    <div className="wo-mod-orders">
      <Outlet />
    </div>
  );
}

export default BikeShopWorkOrdersSection;
