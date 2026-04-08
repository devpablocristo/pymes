import { Outlet } from 'react-router-dom';
import { WorkOrdersHeaderLead } from '../components/WorkOrdersHeaderLead';
import '../pages/WorkOrdersModuleSection.css';

export function BikeShopWorkOrdersSection() {
  return (
    <div className="wo-mod-orders">
      <WorkOrdersHeaderLead
        boardPath="/workshops/bike-shop/orders/board"
        listPath="/workshops/bike-shop/orders/list"
      />
      <Outlet />
    </div>
  );
}

export default BikeShopWorkOrdersSection;
