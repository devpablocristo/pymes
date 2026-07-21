export type CrudContextEntityParams = {
  entity?: string;
  entityId?: string;
};

type SearchParamReader = {
  search: string;
};

export function getCrudSearchParam(
  name: string,
  source?: SearchParamReader | null,
): string | undefined {
  const search =
    source?.search ??
    (typeof window !== 'undefined' ? window.location.search : '');
  return readSearchParam(search, name);
}

function readSearchParam(search: string, name: string): string | undefined {
  const raw = new URLSearchParams(search).get(name);
  const trimmed = raw?.trim();
  return trimmed || undefined;
}

export function getCrudContextEntityParams(
  source?: SearchParamReader | null,
): CrudContextEntityParams {
  return {
    entity: getCrudSearchParam('entity', source),
    entityId: getCrudSearchParam('entity_id', source),
  };
}

export function buildCrudContextEntityPath(
  params: CrudContextEntityParams,
  suffix: string,
): string | null {
  if (!params.entity || !params.entityId) return null;
  return `/v1/${encodeURIComponent(params.entity)}/${encodeURIComponent(params.entityId)}${suffix}`;
}
