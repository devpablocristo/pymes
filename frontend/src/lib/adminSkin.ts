import { createBrowserStorageNamespace } from '@devpablocristo/core-browser/storage';

/** Skin visual de la consola (solo CSS en el host; `modules-crud` y `core-browser` no dependen de esto). */
export type AdminSkinId = 'classic' | 'wowdash';

const STORAGE_KEY = 'pymes:admin-skin';
const DEFAULT_SKIN: AdminSkinId = 'wowdash';

const storage = createBrowserStorageNamespace({ namespace: 'pymes-ui', hostAware: false });

export function getAdminSkin(): AdminSkinId {
  const raw = storage.getString(STORAGE_KEY);
  if (raw === 'classic' || raw === 'wowdash') {
    return raw;
  }
  return DEFAULT_SKIN;
}

export function setAdminSkin(skin: AdminSkinId): void {
  storage.setString(STORAGE_KEY, skin);
  applyAdminSkin(skin);
}

/** Sincroniza `<html data-admin-skin="...">` para que los tokens CSS de la skin activa apliquen. */
export function applyAdminSkin(skin?: AdminSkinId): void {
  const id = skin ?? getAdminSkin();
  document.documentElement.setAttribute('data-admin-skin', id);
}
