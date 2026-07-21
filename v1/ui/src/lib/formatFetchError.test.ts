import { describe, it, expect } from 'vitest';
import { formatBillingPageError } from './formatFetchError';

const UNREACHABLE = 'No se pudo conectar';
const STRIPE_MSG = 'Stripe no configurado';

describe('formatBillingPageError', () => {
  it.each([
    new Error('Failed to fetch'),
    new Error('NetworkError when attempting to fetch resource'),
    new Error('Network request failed'),
    new Error('Load failed'),
    new Error('HttpError: Failed to fetch'),
  ])('classifies network error: %s', (err) => {
    const result = formatBillingPageError(err, UNREACHABLE, STRIPE_MSG);
    expect(result.kind).toBe('error');
    expect(result.message).toBe(UNREACHABLE);
  });

  it.each([
    'Stripe is not configured',
    'billing not configured for this org',
    'HttpError: Stripe not configured',
  ])('classifies stripe unconfigured: %s', (msg) => {
    const result = formatBillingPageError(new Error(msg), UNREACHABLE, STRIPE_MSG);
    expect(result.kind).toBe('stripe_unconfigured');
    expect(result.message).toBe(STRIPE_MSG);
  });

  it('returns cleaned message for generic errors', () => {
    const result = formatBillingPageError(new Error('Something went wrong'), UNREACHABLE, STRIPE_MSG);
    expect(result.kind).toBe('error');
    expect(result.message).toBe('Something went wrong');
  });

  it('strips HttpError prefix from generic errors', () => {
    const result = formatBillingPageError(new Error('HttpError: bad request'), UNREACHABLE, STRIPE_MSG);
    expect(result.message).toBe('bad request');
  });

  it('handles non-Error values', () => {
    const result = formatBillingPageError('some string error', UNREACHABLE, STRIPE_MSG);
    expect(result.kind).toBe('error');
    expect(result.message).toBe('some string error');
  });

  it('returns unreachable message for empty error', () => {
    const result = formatBillingPageError(new Error(''), UNREACHABLE, STRIPE_MSG);
    expect(result.kind).toBe('error');
    expect(result.message).toBe(UNREACHABLE);
  });
});
