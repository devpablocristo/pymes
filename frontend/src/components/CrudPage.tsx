import { useContext, useMemo } from 'react';
import {
  CrudPage as ModulesCrudPage,
  type CrudPageConfig,
} from '@devpablocristo/modules-crud-ui';
import { apiRequest } from '../lib/api';
import { buildPymesCrudStrings } from '../lib/crudModuleStrings';
import { useI18n } from '../lib/i18n';
import { PageSearchShellContext, usePageSearch } from './PageSearch';

export type {
  CrudColumn,
  CrudDataSource,
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

/**
 * CRUD de consola Pymes: motor en `@devpablocristo/modules-crud-ui`, textos vía i18n y API vía `apiRequest`.
 */
export function CrudPage<T extends { id: string }>(props: CrudPageConfig<T>) {
  const { localizeText, sentenceCase, language } = useI18n();
  const stringsBase = useMemo(() => buildPymesCrudStrings(language), [language]);
  const pageSearchInShell = useContext(PageSearchShellContext);
  const pageSearch = usePageSearch();

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
        externalSearch={pageSearchInShell ? pageSearch : undefined}
      />
    </div>
  );
}
