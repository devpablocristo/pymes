import { createBrowserStorageNamespace } from '@devpablocristo/core-browser/storage';

export type AdminSkinId = 'classic' | 'modern';

const STORAGE_KEY = 'pymes:admin-skin';
const DEFAULT_SKIN: AdminSkinId = 'modern';
const storage = createBrowserStorageNamespace({ namespace: 'pymes-ui', hostAware: false });

export function getAdminSkin(): AdminSkinId {
  const raw = storage.getString(STORAGE_KEY);
  if (raw === 'classic' || raw === 'modern') return raw;
  return DEFAULT_SKIN;
}

export function setAdminSkin(skin: AdminSkinId): void {
  storage.setString(STORAGE_KEY, skin);
  applyAdminSkin(skin);
}

export function applyAdminSkin(skin?: AdminSkinId): void {
  const id = skin ?? getAdminSkin();
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('data-admin-skin', id);
  }
}
