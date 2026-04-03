import { Outlet } from 'react-router-dom';
import './WorkOrdersModuleSection.css';

export function WorkOrdersModuleSection() {
  return (
    <div className="wo-mod-orders">
      <Outlet />
    </div>
  );
}
