/** Flags opt-in por recurso; el merge canónico deja todo activo salvo override explícito. */
export type CrudCanonicalFeatureFlags = {
  creatorFilter?: boolean;
  csvToolbar?: boolean;
  /** Reservado: el motor CRUD pagina vía cursor cuando hay `basePath` + cliente HTTP. */
  pagination?: boolean;
  tagsColumn?: boolean;
};

export const defaultCanonicalCrudFeatureFlags: Required<CrudCanonicalFeatureFlags> = {
  creatorFilter: true,
  csvToolbar: true,
  pagination: true,
  tagsColumn: true,
};
