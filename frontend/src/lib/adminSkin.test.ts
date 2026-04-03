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

import { getAdminSkin, setAdminSkin, applyAdminSkin } from './adminSkin';

beforeEach(() => {
  vi.clearAllMocks();
  document.documentElement.removeAttribute('data-admin-skin');
});

describe('getAdminSkin', () => {
  it('returns stored classic skin', () => {
    mockStorage.getString.mockReturnValue('classic');
    expect(getAdminSkin()).toBe('classic');
  });

  it('returns stored wowdash skin', () => {
    mockStorage.getString.mockReturnValue('wowdash');
    expect(getAdminSkin()).toBe('wowdash');
  });

  it('defaults to wowdash for unknown value', () => {
    mockStorage.getString.mockReturnValue('unknown');
    expect(getAdminSkin()).toBe('wowdash');
  });

  it('defaults to wowdash when nothing stored', () => {
    mockStorage.getString.mockReturnValue(null);
    expect(getAdminSkin()).toBe('wowdash');
  });
});

describe('setAdminSkin', () => {
  it('persists and applies classic', () => {
    setAdminSkin('classic');
    expect(mockStorage.setString).toHaveBeenCalledWith('pymes:admin-skin', 'classic');
    expect(document.documentElement.getAttribute('data-admin-skin')).toBe('classic');
  });
});

describe('applyAdminSkin', () => {
  it('sets data-admin-skin attribute', () => {
    applyAdminSkin('wowdash');
    expect(document.documentElement.getAttribute('data-admin-skin')).toBe('wowdash');
  });

  it('uses getAdminSkin default when no argument', () => {
    mockStorage.getString.mockReturnValue('classic');
    applyAdminSkin();
    expect(document.documentElement.getAttribute('data-admin-skin')).toBe('classic');
  });
});
