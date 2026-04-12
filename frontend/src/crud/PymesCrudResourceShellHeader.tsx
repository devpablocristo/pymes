import { mergeCrudStrings, type CrudStrings } from '@devpablocristo/modules-crud-ui';
import { useMemo } from 'react';
import type { ReactNode } from 'react';
import { CrudResourceShellHeader, type CrudResourceShellHeaderConfigLike } from '../modules/crud';
import { buildPymesCrudStrings } from '../lib/crudModuleStrings';
import { useI18n } from '../lib/i18n';
import { useCrudListCreatedByMerge } from '../lib/useCrudListCreatedByMerge';
import { usePymesCrudConfigQuery } from './usePymesCrudConfigQuery';

export function PymesCrudResourceShellHeader<T extends { id: string }>({
  resourceId,
  crudConfigOverride,
  preserveCsvToolbar: _preserveCsvToolbar = false,
  items,
  subtitleCount,
  loading,
  error,
  setError,
  reload,
  searchValue,
  onSearchChange,
  onArchiveToggle,
  searchInlineActions,
  extraHeaderActions,
}: {
  resourceId: string;
  crudConfigOverride?: CrudResourceShellHeaderConfigLike<T> | null;
  preserveCsvToolbar?: boolean;
  items: T[];
  subtitleCount?: number;
  loading: boolean;
  error: string | null;
  setError: (message: string | null) => void;
  reload: () => Promise<void>;
  searchValue: string;
  onSearchChange: (value: string) => void;
  onArchiveToggle?: () => void;
  searchInlineActions?: ReactNode;
  extraHeaderActions?: ReactNode;
}) {
  const { t, localizeText, sentenceCase, language } = useI18n();
  const crudConfigQuery = usePymesCrudConfigQuery<T>(resourceId);
  const crudConfig = (crudConfigOverride ?? crudConfigQuery.data ?? null) as CrudResourceShellHeaderConfigLike<T> | null;
  const { listHeaderInlineSlot } = useCrudListCreatedByMerge();
  const stringsBase = useMemo(() => buildPymesCrudStrings(language), [language]);
  const strings = useMemo<CrudStrings>(() => mergeCrudStrings(stringsBase, {}), [stringsBase]);

  const headerLeadSlot =
    listHeaderInlineSlot &&
    crudConfig?.featureFlags?.headerQuickFilterStrip !== false &&
    crudConfig?.featureFlags?.creatorFilter !== false ? (
      <div className="crud-list-header-lead">{listHeaderInlineSlot({ items })}</div>
    ) : undefined;

  return (
    <CrudResourceShellHeader<T>
      resourceId={resourceId}
      crudConfig={crudConfig}
      strings={strings}
      formatFieldText={localizeText}
      sentenceCase={sentenceCase}
      searchPlaceholder={t('crud.search.placeholder')}
      headerLeadSlot={headerLeadSlot}
      searchInlineActions={searchInlineActions}
      items={items}
      subtitleCount={subtitleCount}
      loading={loading}
      error={error}
      setError={setError}
      reload={reload}
      searchValue={searchValue}
      onSearchChange={onSearchChange}
      onArchiveToggle={onArchiveToggle}
      extraHeaderActions={extraHeaderActions}
    />
  );
}
