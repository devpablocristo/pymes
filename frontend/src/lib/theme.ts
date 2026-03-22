import { createBrowserStorageNamespace } from '@devpablocristo/core-browser/storage';

const STORAGE_KEY = 'pymes:theme';
type Theme = 'light' | 'dark';
const storage = createBrowserStorageNamespace({ namespace: 'pymes-ui', hostAware: false });

export function getTheme(): Theme {
  const stored = storage.getString(STORAGE_KEY);
  if (stored === 'dark' || stored === 'light') return stored;
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function setTheme(theme: Theme): void {
  storage.setString(STORAGE_KEY, theme);
  applyTheme(theme);
}

export function toggleTheme(): Theme {
  const next = getTheme() === 'dark' ? 'light' : 'dark';
  setTheme(next);
  return next;
}

export function applyTheme(theme?: Theme): void {
  const t = theme ?? getTheme();
  document.documentElement.setAttribute('data-theme', t);
}
