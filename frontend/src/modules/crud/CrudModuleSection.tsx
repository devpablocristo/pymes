import { Outlet } from 'react-router-dom';
import { CrudViewModeSwitch } from './CrudViewModeSwitch';
import '../../pages/WorkOrdersModuleSection.css';

type Props = {
  modes: Array<{
    path: string;
    label: string;
    contextPattern?: string;
  }>;
  groupAriaLabel: string;
  description?: string;
  actionLink?: {
    to: string;
    label: string;
    hideWhenActivePattern?: string;
    activeReplacement?: {
      to: string;
      label: string;
    };
  };
};

export function CrudModuleSection(props: Props) {
  return (
    <div className="wo-mod-orders">
      <CrudViewModeSwitch {...props} />
      <Outlet />
    </div>
  );
}
