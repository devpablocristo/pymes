import { Outlet, matchPath, useLocation } from 'react-router-dom';
import type { ReactNode } from 'react';
import { HeaderMenuItemsProvider } from '../../components/HeaderMenuContext';
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
  children?: ReactNode;
};

export function CrudModuleSection(props: Props) {
  const { pathname } = useLocation();
  const isActionHidden = Boolean(
    props.actionLink?.hideWhenActivePattern &&
      matchPath({ path: props.actionLink.hideWhenActivePattern, end: false }, pathname),
  );
  const resolvedActionLink =
    isActionHidden && props.actionLink?.activeReplacement
      ? props.actionLink.activeReplacement
      : isActionHidden
        ? null
        : props.actionLink;
  const menuItems = resolvedActionLink ? [{ label: resolvedActionLink.label, href: resolvedActionLink.to }] : [];

  return (
    <HeaderMenuItemsProvider items={menuItems}>
      <div className="wo-mod-orders">
        <CrudViewModeSwitch {...props} />
        {props.children}
        <Outlet />
      </div>
    </HeaderMenuItemsProvider>
  );
}
