import { describe, it, expect, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  init: vi.fn(),
  captureException: vi.fn(),
}));

vi.mock('@sentry/react', () => ({
  init: mocks.init,
  captureException: mocks.captureException,
}));

// VITE_SENTRY_DSN is not set in test environment, so dsn is undefined
import { initSentry, captureError } from './sentry';

describe('initSentry', () => {
  it('does nothing when VITE_SENTRY_DSN is not set', () => {
    initSentry();
    expect(mocks.init).not.toHaveBeenCalled();
  });
});

describe('captureError', () => {
  it('does nothing when VITE_SENTRY_DSN is not set', () => {
    captureError(new Error('test'));
    expect(mocks.captureException).not.toHaveBeenCalled();
  });

  it('does nothing with context when DSN is not set', () => {
    captureError(new Error('test'), { page: 'billing' });
    expect(mocks.captureException).not.toHaveBeenCalled();
  });
});
