import { useMemo } from 'react';
import {
  CrudPage as ModulesCrudPage,
  type CrudPageConfig as ModulesCrudPageConfig,
} from '@devpablocristo/modules-crud-ui';
import type { CrudCanonicalFeatureFlags } from '../crud/crudFeatureFlags';
import { apiRequest } from '../lib/api';
import { buildPymesCrudStrings } from '../lib/crudModuleStrings';
import { useI18n } from '../lib/i18n';

export type {
  CrudColumn,
  CrudDataSource,
  CrudFieldValue,
  CrudFormField,
  CrudFormValues,
  CrudHelpers,
  CrudHttpClient,
  CrudListHeaderSlotContext,
  CrudRowAction,
  CrudToolbarAction,
} from '@devpablocristo/modules-crud-ui';

export type { CrudCanonicalFeatureFlags };

/** Config CRUD de consola: extiende el motor con flags Pymes (no se reenvían al paquete UI). */
export type CrudPageConfig<T extends { id: string }> = ModulesCrudPageConfig<T> & {
  featureFlags?: CrudCanonicalFeatureFlags;
};

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- mapa heterogéneo: cada config tiene su propio tipo de record, TS no soporta tipos existenciales
export type CrudResourceConfigMap = Record<string, CrudPageConfig<any>>;

/**
 * CRUD de consola Pymes: motor en `@devpablocristo/modules-crud-ui`, textos vía i18n y API vía `apiRequest`.
 */
export function CrudPage<T extends { id: string }>(props: CrudPageConfig<T>) {
  const { featureFlags: _omitFeatureFlags, ...modulesProps } = props;
  const { localizeText, sentenceCase, language } = useI18n();
  const stringsBase = useMemo(() => buildPymesCrudStrings(language), [language]);

  const httpClient = useMemo(
    () =>
      modulesProps.basePath
        ? {
            json: <R,>(path: string, init?: { method?: string; body?: Record<string, unknown> }): Promise<R> =>
              apiRequest<R>(path, {
                method: init?.method as 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE' | undefined,
                body: init?.body,
              }),
          }
        : undefined,
    [modulesProps.basePath],
  );

  return (
    <div className="crud-page-host">
      <ModulesCrudPage
        {...modulesProps}
        stringsBase={stringsBase}
        formatFieldText={localizeText}
        sentenceCase={sentenceCase}
        httpClient={modulesProps.httpClient ?? httpClient}
      />
    </div>
  );
}
