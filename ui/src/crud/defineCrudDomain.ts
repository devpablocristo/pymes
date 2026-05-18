import type { CrudPageConfig, CrudResourceConfigMap } from '../components/CrudPage';
import { withCSVToolbar, type CSVToolbarOptions } from './csvToolbar';
import {
  buildConfiguredCrudPage,
  getCrudPageConfigFromMap,
  hasCrudResourceInMap,
} from './resourceConfigs.runtime';

type CsvOptionsResolver = <T extends { id: string }>(
  resourceId: string,
  config: CrudPageConfig<T>,
) => CSVToolbarOptions;

type DefineCrudDomainOptions = {
  /**
   * Override CSV por recurso. Lo declarado acá sustituye al resolver para ese resourceId.
   * Usar para excepciones (ej. audit con server export dedicado).
   */
  csvOverrides?: Record<string, CSVToolbarOptions>;
  /**
   * Resolver de opciones CSV aplicado a todo recurso sin override.
   * Dominios comerciales/operacionales pasan `mergeCsvOptionsForResource` (dataio si aplica).
   * Dominios sin CSV server (governance, control) omiten este campo → `{}` para cada recurso.
   */
  csvResolver?: CsvOptionsResolver;
};

export function defineCrudDomain(
  resources: CrudResourceConfigMap,
  options: DefineCrudDomainOptions = {},
) {
  const csvOverrides = options.csvOverrides ?? {};
  const csvResolver = options.csvResolver;
  const resourceConfigs = Object.fromEntries(
    Object.entries(resources).map(([resourceId, config]) => {
      const csvOptions = resourceId in csvOverrides
        ? csvOverrides[resourceId]
        : csvResolver?.(resourceId, config) ?? {};
      return [resourceId, withCSVToolbar(resourceId, config, csvOptions)];
    }),
  ) as CrudResourceConfigMap;

  return {
    ConfiguredCrudPage: buildConfiguredCrudPage(resourceConfigs),
    hasCrudResource: (resourceId: string) => hasCrudResourceInMap(resourceConfigs, resourceId),
    getCrudPageConfig: <TRecord extends { id: string } = { id: string }>(resourceId: string) =>
      getCrudPageConfigFromMap<TRecord>(resourceConfigs, resourceId),
  };
}
