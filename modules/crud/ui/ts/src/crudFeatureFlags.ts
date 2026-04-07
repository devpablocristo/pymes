/**
 * Capacidades de consola activables por recurso; el merge canónico asume todo en true salvo override.
 */
export type CrudFeatureFlags = {
  /**
   * Si false, la app no debe inyectar filtros extra en cabecera (p. ej. created_by / Clerk).
   * El propio CrudPage no interpreta este flag; lo consume el shell de la app.
   */
  creatorFilter?: boolean;
  /** Oculta acciones de toolbar CSV conocidas (`csv-export`, `csv-import`). */
  csvToolbar?: boolean;
  /** false = una sola tanda de listado (sin “Cargar más” por cursor). */
  pagination?: boolean;
  /** false = elimina columnas con key `tags`. true + campo form `tags` sin columna → se inyecta una. */
  tagsColumn?: boolean;
};

export const defaultCrudFeatureFlags: Required<CrudFeatureFlags> = {
  creatorFilter: true,
  csvToolbar: true,
  pagination: true,
  tagsColumn: true,
};

export function mergeCrudFeatureFlags(partial?: CrudFeatureFlags): Required<CrudFeatureFlags> {
  return { ...defaultCrudFeatureFlags, ...partial };
}
