import { NavLink, matchPath, useLocation } from 'react-router-dom';
import { HeaderMenu } from '../../components/HeaderMenu';
import { useHeaderMenuItems } from '../../components/HeaderMenuContext';
import '../../styles/viewModeSegmentedSwitch.css';
import '../../pages/WorkOrdersModuleSection.css';

type CrudViewModeLink = {
  path: string;
  label: string;
  contextPattern?: string;
};

type Props = {
  modes: CrudViewModeLink[];
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

export function CrudViewModeSwitch({
  modes,
  groupAriaLabel,
  description,
  actionLink,
}: Props) {
  const { pathname } = useLocation();
  const contextualMenuItems = useHeaderMenuItems();
  function isModeActive(mode: CrudViewModeLink): boolean {
    return Boolean(
      matchPath({ path: mode.path, end: true }, pathname) ||
        (mode.contextPattern && matchPath({ path: mode.contextPattern, end: false }, pathname)),
    );
  }

  return (
    <div className="wo-mod-orders__header-lead">
      {description ? <p>{description}</p> : null}
      <div className="wo-mod-orders__bar">
        <nav className="m-view-tabs" aria-label={groupAriaLabel}>
          {modes.map((mode) => (
            <NavLink
              key={mode.path}
              to={mode.path}
              draggable={false}
              className={`m-view-tabs__item${isModeActive(mode) ? ' m-view-tabs__item--active' : ''}`}
            >
              {mode.label}
            </NavLink>
          ))}
        </nav>
        <HeaderMenu items={contextualMenuItems} />
      </div>
    </div>
  );
}
