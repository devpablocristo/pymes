import { CrudPageShell } from '@devpablocristo/core-browser/crud';
import {
  CrudShellHeaderActionsColumn,
  interpolate,
  type CrudStrings,
  type CrudToolbarAction,
} from '@devpablocristo/modules-crud-ui';
import { useMemo } from 'react';
import type { ReactNode } from 'react';
import { CrudArchivedSearchParamToggle } from './CrudArchivedSearchParamToggle';
import { CrudToolbarActionButtons } from './CrudToolbarActionButtons';
import { useCrudArchivedSearchParam } from './useCrudArchivedSearchParam';
import './CrudResourceShellHeader.css';

export type CrudResourceShellHeaderConfigLike<T extends { id: string }> = {
  label?: string;
  labelPlural?: string;
  labelPluralCap?: string;
  searchPlaceholder?: string;
  toolbarActions?: CrudToolbarAction<T>[];
  supportsArchived?: boolean;
  featureFlags?: {
    headerQuickFilterStrip?: boolean;
    creatorFilter?: boolean;
    statusSelector?: boolean;
  };
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

  const headerActionsResolved = (
    <CrudShellHeaderActionsColumn
      search={{
        value: searchValue,
        onChange: onSearchChange,
        placeholder: searchPlaceholder ?? str.searchPlaceholder,
        inputClassName: 'm-kanban__search crud-resource-shell-header__search',
      }}
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
      {crudConfig?.supportsArchived ? (
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
  );

  return (
    <div className="crud-resource-shell-header">
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
