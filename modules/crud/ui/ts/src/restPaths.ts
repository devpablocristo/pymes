/**
 * Segmentos de URL para el modo `basePath` de CrudPage.
 * Deben coincidir con `github.com/devpablocristo/modules/crud/paths/go` (constantes Segment*).
 */
export const CrudPathSegment = {
  archived: "archived",
  archive: "archive",
  restore: "restore",
  hard: "hard",
} as const;

function trimTrailingSlash(path: string): string {
  return path.replace(/\/+$/, "");
}

/** GET colección: activos o `/archived`. */
export function crudListPath(basePath: string, archived: boolean): string {
  const base = trimTrailingSlash(basePath);
  return archived ? `${base}/${CrudPathSegment.archived}` : base;
}

/** Recurso por id; `suffix` para restore/hard. */
export function crudItemPath(basePath: string, id: string, suffix?: "restore" | "hard"): string {
  const base = trimTrailingSlash(basePath);
  if (suffix === "restore") {
    return `${base}/${id}/${CrudPathSegment.restore}`;
  }
  if (suffix === "hard") {
    return `${base}/${id}/${CrudPathSegment.hard}`;
  }
  return `${base}/${id}`;
}
