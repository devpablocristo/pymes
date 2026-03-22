/**
 * Convierte errores de fetch en texto entendible (evita "TypeError: Failed to fetch" crudo).
 */
export function formatFetchErrorForUser(err: unknown, unreachableMessage: string): string {
  const msg = err instanceof Error ? err.message : String(err);
  if (/failed to fetch|networkerror|network request failed|load failed/i.test(msg)) {
    return unreachableMessage;
  }
  return stripHttpErrorPrefix(msg);
}

/** Quita el prefijo que a veces añade el cliente HTTP (evita "HttpError: ..." en pantalla). */
export function stripHttpErrorPrefix(message: string): string {
  return message.replace(/^HttpError:\s*/i, '').trim();
}

export type BillingPageErrorKind = 'stripe_unconfigured' | 'error';

/**
 * Clasifica errores de /v1/billing/* para mostrar copy de producto en lugar del mensaje técnico del core.
 */
export function formatBillingPageError(
  err: unknown,
  unreachableMessage: string,
  stripeNotConfiguredMessage: string,
): { kind: BillingPageErrorKind; message: string } {
  const raw = err instanceof Error ? err.message : String(err);
  const clean = stripHttpErrorPrefix(raw);
  if (/failed to fetch|networkerror|network request failed|load failed/i.test(clean)) {
    return { kind: 'error', message: unreachableMessage };
  }
  const low = clean.toLowerCase();
  if ((low.includes('stripe') || low.includes('billing')) && low.includes('not configured')) {
    return { kind: 'stripe_unconfigured', message: stripeNotConfiguredMessage };
  }
  return { kind: 'error', message: clean || unreachableMessage };
}
