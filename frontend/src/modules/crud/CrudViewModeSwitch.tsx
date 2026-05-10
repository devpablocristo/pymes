import { NavLink, matchPath, useLocation } from 'react-router-dom';

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

export function CrudViewModeSwitch(props: Props) {
  const { pathname } = useLocation();

  if (props.modes.length <= 1) {
    return null;
  }

  function isModeActive(mode: CrudViewModeLink): boolean {
    return Boolean(
      matchPath({ path: mode.path, end: true }, pathname) ||
        (mode.contextPattern && matchPath({ path: mode.contextPattern, end: false }, pathname)),
    );
  }

  return (
    <div className="wo-mod-orders__header-lead">
      {props.description ? <p>{props.description}</p> : null}
      <div className="wo-mod-orders__bar">
        <nav className="m-view-tabs" aria-label={props.groupAriaLabel}>
          {props.modes.map((mode) => (
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
        {props.actionLink ? (
          <NavLink className="wo-mod-orders__action" to={props.actionLink.to}>
            {props.actionLink.label}
          </NavLink>
        ) : null}
      </div>
    </div>
  );
}
