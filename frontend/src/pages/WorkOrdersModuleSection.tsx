import { Outlet } from 'react-router-dom';
import { WorkOrdersHeaderLead } from '../components/WorkOrdersHeaderLead';
import './WorkOrdersModuleSection.css';

export function WorkOrdersModuleSection() {
  return (
    <div className="wo-mod-orders">
      <WorkOrdersHeaderLead boardPath="/modules/carWorkOrders/board" listPath="/modules/carWorkOrders/list" />
      <Outlet />
    </div>
  );
}
