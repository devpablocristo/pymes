import { useMemo } from 'react';
import { CrudPage as ModulesCrudPage, type CrudPageConfig } from '@devpablocristo/modules-crud-ui';
import { apiRequest } from '../lib/api';
import { buildPymesCrudStrings } from '../lib/crudModuleStrings';
import { useI18n } from '../lib/i18n';

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
  CrudPageConfig,
  CrudRowAction,
  CrudToolbarAction,
} from '@devpablocristo/modules-crud-ui';

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
