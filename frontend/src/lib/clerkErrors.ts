import { isClerkAPIResponseError } from '@clerk/react/errors';

/**
 * Mensaje legible para UI a partir de errores de la API de Clerk (crear org, update, etc.).
 * No exponer detalles crudos salvo los que Clerk ya formatea para el usuario.
 */
export function formatClerkAPIUserMessage(err: unknown, fallback: string): string {
  if (isClerkAPIResponseError(err)) {
    const first = err.errors?.[0];
    const msg = first && typeof first === 'object' && 'message' in first ? String(first.message) : '';
    if (msg.trim()) {
      return msg.trim();
    }
  }
  if (err instanceof Error && err.message.trim()) {
    return err.message.trim();
  }
  return fallback;
}
