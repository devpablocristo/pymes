// Re-export agnostic helpers from core
export { formatFetchErrorForUser, stripHttpErrorPrefix } from '@devpablocristo/core-http/errors';

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
  const clean = raw.replace(/^HttpError:\s*/i, '').trim();
  if (/failed to fetch|networkerror|network request failed|load failed/i.test(clean)) {
    return { kind: 'error', message: unreachableMessage };
  }
  const low = clean.toLowerCase();
  if ((low.includes('stripe') || low.includes('billing')) && low.includes('not configured')) {
    return { kind: 'stripe_unconfigured', message: stripeNotConfiguredMessage };
  }
  return { kind: 'error', message: clean || unreachableMessage };
}
