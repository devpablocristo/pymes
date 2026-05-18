import { describe, it, expect, vi } from 'vitest';

vi.mock('@clerk/react/errors', () => ({
  isClerkAPIResponseError: (err: unknown) =>
    err !== null && typeof err === 'object' && '__clerkApi' in err,
}));

import { formatClerkAPIUserMessage } from './clerkErrors';

describe('formatClerkAPIUserMessage', () => {
  const DEFAULT_MESSAGE = 'Algo sali\u00f3 mal';

  it('extracts message from Clerk API error', () => {
    const err = {
      __clerkApi: true,
      errors: [{ message: 'Organization name already taken' }],
    };
    expect(formatClerkAPIUserMessage(err, DEFAULT_MESSAGE)).toBe('Organization name already taken');
  });

  it('trims Clerk error message', () => {
    const err = {
      __clerkApi: true,
      errors: [{ message: '  spaced message  ' }],
    };
    expect(formatClerkAPIUserMessage(err, DEFAULT_MESSAGE)).toBe('spaced message');
  });

  it('falls through to Error.message when Clerk error has no message', () => {
    const err = {
      __clerkApi: true,
      errors: [{ message: '' }],
    };
    expect(formatClerkAPIUserMessage(err, DEFAULT_MESSAGE)).toBe(DEFAULT_MESSAGE);
  });

  it('uses Error.message for non-Clerk errors', () => {
    expect(formatClerkAPIUserMessage(new Error('custom error'), DEFAULT_MESSAGE)).toBe('custom error');
  });

  it('returns default message for non-Error, non-Clerk values', () => {
    expect(formatClerkAPIUserMessage('string error', DEFAULT_MESSAGE)).toBe(DEFAULT_MESSAGE);
    expect(formatClerkAPIUserMessage(null, DEFAULT_MESSAGE)).toBe(DEFAULT_MESSAGE);
    expect(formatClerkAPIUserMessage(undefined, DEFAULT_MESSAGE)).toBe(DEFAULT_MESSAGE);
  });

  it('returns default message when Error message is empty', () => {
    expect(formatClerkAPIUserMessage(new Error(''), DEFAULT_MESSAGE)).toBe(DEFAULT_MESSAGE);
    expect(formatClerkAPIUserMessage(new Error('   '), DEFAULT_MESSAGE)).toBe(DEFAULT_MESSAGE);
  });
});
