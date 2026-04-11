import { useMemo, type ReactElement } from 'react';
import {
  CrudPage as ModulesCrudPage,
  type CrudPageConfig as ModulesCrudPageConfig,
  type CrudViewModeConfig as ModulesCrudViewModeConfig,
  type CrudViewModeId as ModulesCrudViewModeId,
} from '@devpablocristo/modules-crud-ui';
import { apiRequest } from '../lib/api';
import { buildPymesCrudStrings } from '../lib/crudModuleStrings';
import { useI18n } from '../lib/i18n';

export type CrudViewModeId = ModulesCrudViewModeId;
export type CrudViewModeConfig = ModulesCrudViewModeConfig & {
  /** Vista no-lista definida junto al recurso en `resourceConfigs.*`. */
  render?: () => ReactElement;
};

export type CrudExplorerMetricConfig<T> = {
  id: string;
  label: string;
  value: (items: T[]) => string;
  tone?: 'default' | 'success' | 'warning' | 'danger';
  helper?: string | ((items: T[]) => string | undefined);
};

export type CrudExplorerFilterConfig<T> = {
  id: string;
  label: string;
  predicate: (row: T) => boolean;
};

export type CrudExplorerDetailConfig<T extends { id: string }> = {
  title: string;
  emptyState?: string;
  metrics?: CrudExplorerMetricConfig<T>[];
  filters?: CrudExplorerFilterConfig<T>[];
  renderDetail: (row: T, ctx: { items: T[]; reload: () => Promise<void> }) => import('react').ReactNode;
};

export type {
  CrudColumn,
  CrudDataSource,
  CrudFeatureFlags,
  CrudFieldValue,
  CrudFormField,
  CrudFormValues,
  CrudHelpers,
  CrudHttpClient,
  CrudListHeaderSlotContext,
  CrudRowAction,
  CrudToolbarAction,
} from '@devpablocristo/modules-crud-ui';

export type CrudPageConfig<T extends { id: string }> = ModulesCrudPageConfig<T> & {
  viewModes?: CrudViewModeConfig[];
  explorerDetail?: CrudExplorerDetailConfig<T>;
  /** Extensión Pymes: render de celda tags cuando el módulo CRUD lo soporta vía CSV/flags. */
  renderTagsCell?: (row: T) => import('react').ReactNode;
};

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- mapa heterogéneo: cada config tiene su propio tipo de record, TS no soporta tipos existenciales
export type CrudResourceConfigMap = Record<string, CrudPageConfig<any>>;

/**
 * CRUD de consola Pymes: motor en `@devpablocristo/modules-crud-ui`, textos vía i18n y API vía `apiRequest`.
 */
export function CrudPage<T extends { id: string }>(props: CrudPageConfig<T>) {
  const { localizeText, sentenceCase, language } = useI18n();
  const stringsBase = useMemo(() => buildPymesCrudStrings(language), [language]);

  const httpClient = useMemo(
    () =>
      props.basePath
        ? {
            json: <R,>(path: string, init?: { method?: string; body?: Record<string, unknown> }): Promise<R> =>
              apiRequest<R>(path, {
                method: init?.method as 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE' | undefined,
                body: init?.body,
              }),
          }
        : undefined,
    [props.basePath],
  );

  return (
    <div className="crud-page-host">
      <ModulesCrudPage
        {...props}
        stringsBase={stringsBase}
        formatFieldText={localizeText}
        sentenceCase={sentenceCase}
        httpClient={props.httpClient ?? httpClient}
      />
    </div>
  );
}
