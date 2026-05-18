import { createThemeManager } from '@devpablocristo/core-browser/theme';

const themeManager = createThemeManager({
  namespace: 'pymes-ui',
  storageKey: 'pymes:theme',
});

export const getTheme = themeManager.get;
export const toggleTheme = themeManager.toggle;
export const applyTheme = themeManager.apply;
