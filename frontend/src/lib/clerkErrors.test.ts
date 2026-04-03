import { describe, it, expect, vi } from 'vitest';

vi.mock('@clerk/react/errors', () => ({
  isClerkAPIResponseError: (err: unknown) =>
    err !== null && typeof err === 'object' && '__clerkApi' in err,
}));

import { formatClerkAPIUserMessage } from './clerkErrors';

describe('formatClerkAPIUserMessage', () => {
  const FALLBACK = 'Algo sali\u00f3 mal';

  it('extracts message from Clerk API error', () => {
    const err = {
      __clerkApi: true,
      errors: [{ message: 'Organization name already taken' }],
    };
    expect(formatClerkAPIUserMessage(err, FALLBACK)).toBe('Organization name already taken');
  });

  it('trims Clerk error message', () => {
    const err = {
      __clerkApi: true,
      errors: [{ message: '  spaced message  ' }],
    };
    expect(formatClerkAPIUserMessage(err, FALLBACK)).toBe('spaced message');
  });

  it('falls through to Error.message when Clerk error has no message', () => {
    const err = {
      __clerkApi: true,
      errors: [{ message: '' }],
    };
    // Not an Error instance, so should hit fallback
    expect(formatClerkAPIUserMessage(err, FALLBACK)).toBe(FALLBACK);
  });

  it('uses Error.message for non-Clerk errors', () => {
    expect(formatClerkAPIUserMessage(new Error('custom error'), FALLBACK)).toBe('custom error');
  });

  it('returns fallback for non-Error, non-Clerk values', () => {
    expect(formatClerkAPIUserMessage('string error', FALLBACK)).toBe(FALLBACK);
    expect(formatClerkAPIUserMessage(null, FALLBACK)).toBe(FALLBACK);
    expect(formatClerkAPIUserMessage(undefined, FALLBACK)).toBe(FALLBACK);
  });

  it('returns fallback when Error message is empty', () => {
    expect(formatClerkAPIUserMessage(new Error(''), FALLBACK)).toBe(FALLBACK);
    expect(formatClerkAPIUserMessage(new Error('   '), FALLBACK)).toBe(FALLBACK);
  });
});
