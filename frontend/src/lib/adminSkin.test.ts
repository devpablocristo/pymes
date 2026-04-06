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

  it('returns stored modern skin', () => {
    mockStorage.getString.mockReturnValue('modern');
    expect(getAdminSkin()).toBe('modern');
  });

  it('defaults to modern for unknown value', () => {
    mockStorage.getString.mockReturnValue('unknown');
    expect(getAdminSkin()).toBe('modern');
  });

  it('defaults to modern when nothing stored', () => {
    mockStorage.getString.mockReturnValue(null);
    expect(getAdminSkin()).toBe('modern');
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
    applyAdminSkin('modern');
    expect(document.documentElement.getAttribute('data-admin-skin')).toBe('modern');
  });

  it('uses getAdminSkin default when no argument', () => {
    mockStorage.getString.mockReturnValue('classic');
    applyAdminSkin();
    expect(document.documentElement.getAttribute('data-admin-skin')).toBe('classic');
  });
});
