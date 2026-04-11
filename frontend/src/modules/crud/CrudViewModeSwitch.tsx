import { NavLink, useMatch } from 'react-router-dom';
import '../../styles/viewModeSegmentedSwitch.css';
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

export function CrudViewModeSwitch({
  primaryPath,
  secondaryPath,
  primaryLabel,
  secondaryLabel,
  groupAriaLabel,
  secondaryContextPattern,
  description,
}: Props) {
  const isPrimaryActive = useMatch(primaryPath);
  const isSecondaryMatch = useMatch(secondaryPath);
  const isSecondaryContextMatch = useMatch(secondaryContextPattern ?? `${secondaryPath}/*`);
  const isSecondaryContext = isSecondaryMatch || isSecondaryContextMatch;

  return (
    <div className="wo-mod-orders__header-lead">
      {description ? <p>{description}</p> : null}
      <div className="m-seg-switch" role="group" aria-label={groupAriaLabel}>
        <NavLink to={primaryPath} className="m-seg-switch__track" draggable={false}>
          <span className={`m-seg-switch__label${isPrimaryActive ? ' m-seg-switch__label--active' : ''}`}>
            {primaryLabel}
          </span>
          <span className={`m-seg-switch__label${!isPrimaryActive && isSecondaryContext ? ' m-seg-switch__label--active' : ''}`}>
            {secondaryLabel}
          </span>
          <span
            className={`m-seg-switch__thumb${isPrimaryActive ? ' m-seg-switch__thumb--left' : ' m-seg-switch__thumb--right'}`}
          />
        </NavLink>
        {!isPrimaryActive ? (
          <NavLink
            to={primaryPath}
            className="m-seg-switch__hit m-seg-switch__hit--left"
            aria-hidden="true"
            draggable={false}
            tabIndex={-1}
          >
            &nbsp;
          </NavLink>
        ) : null}
        {isPrimaryActive ? (
          <NavLink
            to={secondaryPath}
            className="m-seg-switch__hit m-seg-switch__hit--right"
            aria-hidden="true"
            draggable={false}
            tabIndex={-1}
          >
            &nbsp;
          </NavLink>
        ) : null}
      </div>
    </div>
  );
}
