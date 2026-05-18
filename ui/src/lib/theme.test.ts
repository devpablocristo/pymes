import { describe, it, expect, vi, beforeEach } from 'vitest';

const mockStorage = vi.hoisted(() => ({
  getJSON: vi.fn(),
  setJSON: vi.fn(),
  remove: vi.fn(),
  getString: vi.fn(),
  setString: vi.fn(),
}));

vi.mock('@devpablocristo/core-browser/storage', () => ({
  createBrowserStorageNamespace: () => mockStorage,
}));

import { getTheme, toggleTheme, applyTheme } from './theme';

beforeEach(() => {
  vi.clearAllMocks();
  document.documentElement.removeAttribute('data-theme');
});

describe('getTheme', () => {
  it('returns stored dark theme', () => {
    mockStorage.getString.mockReturnValue('dark');
    expect(getTheme()).toBe('dark');
  });

  it('returns stored light theme', () => {
    mockStorage.getString.mockReturnValue('light');
    expect(getTheme()).toBe('light');
  });

  it('falls back to system preference when nothing stored', () => {
    mockStorage.getString.mockReturnValue(null);
    // jsdom doesn't provide matchMedia by default, so stub it
    window.matchMedia = vi.fn().mockReturnValue({ matches: false }) as unknown as typeof window.matchMedia;
    expect(getTheme()).toBe('light');
  });

  it('returns dark when system prefers dark', () => {
    mockStorage.getString.mockReturnValue(null);
    window.matchMedia = vi.fn().mockReturnValue({ matches: true }) as unknown as typeof window.matchMedia;
    expect(getTheme()).toBe('dark');
  });
});

describe('toggleTheme', () => {
  it('toggles from dark to light', () => {
    mockStorage.getString.mockReturnValue('dark');
    const result = toggleTheme();
    expect(result).toBe('light');
    expect(mockStorage.setString).toHaveBeenCalledWith('pymes:theme', 'light');
  });

  it('toggles from light to dark', () => {
    mockStorage.getString.mockReturnValue('light');
    const result = toggleTheme();
    expect(result).toBe('dark');
    expect(mockStorage.setString).toHaveBeenCalledWith('pymes:theme', 'dark');
  });
});

describe('applyTheme', () => {
  it('sets data-theme attribute on documentElement', () => {
    applyTheme('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });

  it('uses getTheme default when no argument', () => {
    mockStorage.getString.mockReturnValue('light');
    applyTheme();
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
  });
});
