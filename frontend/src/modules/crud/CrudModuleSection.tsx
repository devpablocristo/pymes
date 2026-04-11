import { Outlet } from 'react-router-dom';
import { CrudViewModeSwitch } from './CrudViewModeSwitch';
import '../../pages/WorkOrdersModuleSection.css';

type Props = {
  primaryPath: string;
  secondaryPath: string;
  primaryLabel: string;
  secondaryLabel: string;
  groupAriaLabel: string;
  secondaryContextPattern?: string;
  description?: string;
};

export function CrudModuleSection(props: Props) {
  return (
    <div className="wo-mod-orders">
      <CrudViewModeSwitch {...props} />
      <Outlet />
    </div>
  );
}
