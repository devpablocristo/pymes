import { CrudPageShell } from '@devpablocristo/core-browser/crud';
import {
  CrudShellHeaderActionsColumn,
  interpolate,
  mergeCrudStrings,
  type CrudToolbarAction,
} from '@devpablocristo/modules-crud-ui';
import { useMemo } from 'react';
import { buildPymesCrudStrings } from '../../lib/crudModuleStrings';
import { useI18n } from '../../lib/i18n';
import { useCrudListCreatedByMerge } from '../../lib/useCrudListCreatedByMerge';
import { CrudArchivedSearchParamToggle } from './CrudArchivedSearchParamToggle';
import { CrudToolbarActionButtons } from './CrudToolbarActionButtons';
import { useCrudArchivedSearchParam } from './useCrudArchivedSearchParam';
import { useCrudConfigQuery } from './useCrudConfigQuery';
import './CrudResourceShellHeader.css';

export type CrudResourceShellHeaderProps<T extends { id: string }> = {
  /** Id de recurso CRUD (`resourceConfigs.*`), sin acoplar a un dominio vertical. */
  resourceId: string;
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
  /** Igual que `loadLazyCrudPageConfig` / `useCrudConfigQuery` para vistas custom. */
  preserveCsvToolbar?: boolean;
};

/**
 * Cabecera de consola para un recurso CRUD: `CrudPageShell` + `CrudShellHeaderActionsColumn` (paridad con `CrudPage` del paquete).
 * Agnóstico de negocio: el dominio entra solo por `resourceId` y el tipo de fila `T`.
 */
export function CrudResourceShellHeader<T extends { id: string }>({
  resourceId,
  items,
  subtitleCount,
  loading,
  error,
  setError,
  reload,
  searchValue,
  onSearchChange,
  onArchiveToggle,
  preserveCsvToolbar = false,
}: CrudResourceShellHeaderProps<T>) {
  const { t, localizeText, sentenceCase, language } = useI18n();
  const { archived: showArchived } = useCrudArchivedSearchParam();
  const crudConfigQuery = useCrudConfigQuery<T>(resourceId, { preserveCsvToolbar });
  const crudConfig = crudConfigQuery.data ?? null;
  const { listHeaderInlineSlot } = useCrudListCreatedByMerge();

  const stringsBase = useMemo(() => buildPymesCrudStrings(language), [language]);
  const str = useMemo(() => mergeCrudStrings(stringsBase, {}), [stringsBase]);

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
  const labelOne = vars.label.trim() || t('crud.resource.rowLabel');
  const labelMany = vars.labelPlural.trim() || t('crud.resource.rowsLabel');
  const subtitle = loading
    ? str.statusLoading
    : `${count} ${count === 1 ? labelOne : labelMany}`;

  const toolbarActions = (crudConfig?.toolbarActions ?? []) as CrudToolbarAction<T>[];

  const headerActionsResolved = (
    <CrudShellHeaderActionsColumn
      search={{
        value: searchValue,
        onChange: onSearchChange,
        placeholder: t('crud.search.placeholder'),
        inputClassName: 'm-kanban__search crud-resource-shell-header__search',
      }}
    >
      <CrudToolbarActionButtons
        actions={toolbarActions}
        items={items}
        archived={showArchived}
        reload={reload}
        setError={setError}
        formatLabel={localizeText}
      />
      <CrudArchivedSearchParamToggle
        className="btn-secondary btn-sm"
        showActiveLabel={str.toggleShowActive}
        showArchivedLabel={str.toggleShowArchived}
        onToggle={() => {
          onArchiveToggle?.();
        }}
      />
    </CrudShellHeaderActionsColumn>
  );

  return (
    <div className="crud-resource-shell-header">
      <CrudPageShell
        title={showArchived ? titleArchivedView : titleActive}
        subtitle={subtitle}
        headerLeadSlot={
          listHeaderInlineSlot &&
          crudConfig?.featureFlags?.headerQuickFilterStrip !== false &&
          crudConfig?.featureFlags?.creatorFilter !== false ? (
            <div className="crud-list-header-lead">{listHeaderInlineSlot({ items })}</div>
          ) : undefined
        }
        headerActions={headerActionsResolved}
        error={error ? <div className="alert alert-error">{error}</div> : undefined}
      >
        {null}
      </CrudPageShell>
    </div>
  );
}
