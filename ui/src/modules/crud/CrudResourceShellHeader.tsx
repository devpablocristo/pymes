import { CrudPageShell } from '@devpablocristo/platform-browser/crud';
import {
  CrudShellHeaderActionsColumn,
  interpolate,
  type CrudStrings,
  type CrudToolbarAction,
} from '@devpablocristo/platform-crud-ui';
import { useMemo } from 'react';
import type { ReactNode } from 'react';
import { NavLink, matchPath, useLocation, useNavigate } from 'react-router-dom';
import { CrudArchivedSearchParamToggle } from './CrudArchivedSearchParamToggle';
import { CrudToolbarActionButtons } from './CrudToolbarActionButtons';
import { useCrudArchivedSearchParam } from './useCrudArchivedSearchParam';
import { useViewModes } from './ViewModeTabsCtx';
import type { CrudStateMachineConfig } from '../../components/CrudPage';
import { HeaderMenu } from '../../components/HeaderMenu';
import { useHeaderMenuItems } from '../../components/useHeaderMenuItems';
import { NotificationsDropdown } from '../../components/NotificationsDropdown';
import { tenantLink, useTenantSlug } from '../../lib/tenantSlug';
import './CrudResourceShellHeader.css';

export type CrudResourceShellHeaderConfigLike<T extends { id: string }> = {
  label?: string;
  labelPlural?: string;
  labelPluralCap?: string;
  searchPlaceholder?: string;
  toolbarActions?: CrudToolbarAction<T>[];
  supportsArchived?: boolean;
  featureFlags?: {
    searchBar?: boolean;
    headerQuickFilterStrip?: boolean;
    creatorFilter?: boolean;
    archivedToggle?: boolean;
    createAction?: boolean;
  };
  stateMachine?: CrudStateMachineConfig<T>;
};

export type CrudResourceShellHeaderProps<T extends { id: string }> = {
  /** Id de recurso CRUD (`resourceConfigs.*`), sin acoplar a un dominio vertical. */
  resourceId: string;
  crudConfig: CrudResourceShellHeaderConfigLike<T> | null;
  strings: CrudStrings;
  formatFieldText?: (value: string) => string;
  sentenceCase?: (value: string) => string;
  searchPlaceholder?: string;
  headerLeadSlot?: React.ReactNode;
  searchInlineActions?: ReactNode;
  extraHeaderActions?: ReactNode;
  items: T[];
  /** Conteo del subtítulo (p. ej. filas visibles); por defecto `items.length`. */
  subtitleCount?: number;
  loading: boolean;
  error: string | null;
  setError: (message: string | null) => void;
  reload: () => Promise<void>;
  searchValue: string;
  onSearchChange: (value: string) => void;
  onArchiveToggle?: () => void;
};

/**
 * Cabecera de consola para un recurso CRUD: `CrudPageShell` + `CrudShellHeaderActionsColumn` (paridad con `CrudPage` del paquete).
 * Agnóstico de negocio: el dominio entra solo por `resourceId` y el tipo de fila `T`.
 */
export function CrudResourceShellHeader<T extends { id: string }>({
  resourceId,
  crudConfig,
  strings,
  formatFieldText = (value) => value,
  sentenceCase = (value) => value,
  searchPlaceholder,
  headerLeadSlot,
  searchInlineActions,
  extraHeaderActions,
  items,
  subtitleCount,
  loading,
  error,
  setError,
  reload,
  searchValue,
  onSearchChange,
  onArchiveToggle,
}: CrudResourceShellHeaderProps<T>) {
  const { archived: showArchived } = useCrudArchivedSearchParam();
  const str = strings;
  const navigate = useNavigate();
  const slug = useTenantSlug();
  const contextualMenuItems = useHeaderMenuItems();
  const viewModes = useViewModes();
  const { pathname } = useLocation();

  function isModeActive(mode: { path: string; contextPattern?: string }): boolean {
    return Boolean(
      matchPath({ path: mode.path, end: true }, pathname) ||
        (mode.contextPattern && matchPath({ path: mode.contextPattern, end: false }, pathname)),
    );
  }

  const vars = useMemo(
    () => ({
      label: crudConfig?.label ?? '',
      labelPlural: crudConfig?.labelPlural ?? '',
      labelPluralCap: crudConfig?.labelPluralCap ?? '',
    }),
    [crudConfig],
  );

  const labelPluralCapSafe = (crudConfig?.labelPluralCap ?? '').trim();
  const titleActive = labelPluralCapSafe ? sentenceCase(labelPluralCapSafe) : resourceId;
  const titleArchivedView = labelPluralCapSafe ? sentenceCase(interpolate(str.titleArchived, vars)) : titleActive;

  const count = subtitleCount ?? items.length;
  const labelOne = vars.label.trim() || 'item';
  const labelMany = vars.labelPlural.trim() || 'items';
  const subtitle = loading
    ? str.statusLoading
    : `${count} ${count === 1 ? labelOne : labelMany}`;

  const toolbarActions = (crudConfig?.toolbarActions ?? []) as CrudToolbarAction<T>[];
  const searchEnabled = crudConfig?.featureFlags?.searchBar !== false;
  const archivedToggleEnabled = crudConfig?.featureFlags?.archivedToggle !== false;

  const viewTabsNode =
    viewModes && viewModes.length > 1 ? (
      <nav className="m-view-tabs" aria-label="Vista">
        {viewModes.map((mode) => (
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
    ) : null;

  const headerActionsResolved = (
    <>
      {viewTabsNode}
      <CrudShellHeaderActionsColumn
      search={
        searchEnabled
          ? {
              value: searchValue,
              onChange: onSearchChange,
              placeholder: searchPlaceholder ?? str.searchPlaceholder,
              inputClassName: 'm-kanban__search crud-resource-shell-header__search',
            }
          : undefined
      }
      searchInlineActions={searchInlineActions}
    >
      <CrudToolbarActionButtons
        actions={toolbarActions}
        items={items}
        archived={showArchived}
        reload={reload}
        setError={setError}
        formatLabel={formatFieldText}
      />
      {crudConfig?.supportsArchived && archivedToggleEnabled ? (
        <CrudArchivedSearchParamToggle
          className="btn-secondary btn-sm"
          showActiveLabel={str.toggleShowActive}
          showArchivedLabel={str.toggleShowArchived}
          onToggle={() => {
            onArchiveToggle?.();
          }}
        />
      ) : null}
      {extraHeaderActions}
    </CrudShellHeaderActionsColumn>
    </>
  );

  return (
    <div className="crud-resource-shell-header page-stack">
      <div className="page-layout__header-top-row">
        <div className="topbar-actions">
          {contextualMenuItems.map((item) => (
            <button
              key={`${item.label}:${item.href}`}
              type="button"
              className="topbar-icon-btn"
              aria-label={item.label}
              title={item.label}
              onClick={() => {
                item.onSelect?.();
                navigate(item.href);
              }}
            >
              <i className={`ti ti-${topbarContextualIcon(item.label)}`} aria-hidden="true" />
            </button>
          ))}
          <NotificationsDropdown />
          <button
            type="button"
            className="topbar-icon-btn"
            aria-label="Configuración"
            onClick={() => navigate(tenantLink('/settings', slug))}
          >
            <i className="ti ti-settings" aria-hidden="true" />
          </button>
        </div>
        <HeaderMenu items={contextualMenuItems} />
      </div>
      <CrudPageShell
        title={showArchived ? titleArchivedView : titleActive}
        subtitle={subtitle}
        headerLeadSlot={headerLeadSlot}
        headerActions={headerActionsResolved}
        error={error ? <div className="alert alert-error">{error}</div> : undefined}
      >
        {null}
      </CrudPageShell>
    </div>
  );
}

function topbarContextualIcon(label: string): string {
  const normalized = label.trim().toLowerCase();
  if (normalized.startsWith('configurar')) {
    return 'adjustments-horizontal';
  }
  if (normalized.startsWith('volver')) {
    return 'arrow-left';
  }
  return 'dots';
}
