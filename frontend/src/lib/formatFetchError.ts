/**
 * Convierte errores de fetch en texto entendible (evita "TypeError: Failed to fetch" crudo).
 */
export function formatFetchErrorForUser(err: unknown, unreachableMessage: string): string {
  const msg = err instanceof Error ? err.message : String(err);
  if (/failed to fetch|networkerror|network request failed|load failed/i.test(msg)) {
    return unreachableMessage;
  }
  return msg;
}
