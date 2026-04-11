import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig, searchForWorkspaceRoot } from 'vite';
import { configDefaults } from 'vitest/config';
import react from '@vitejs/plugin-react';

/** Repo pymes (directorio que contiene `frontend/`). */
const pymesRepoRoot = fileURLToPath(new URL('..', import.meta.url));
/** Directorio padre del repo (core/modules como hermanos de pymes — layout local típico). */
const pymesParentDir = fileURLToPath(new URL('../..', import.meta.url));

/**
 * Resuelve `core/...` desde `pymes/core` (anidado) o repo hermano `../core`.
 */
function monorepoCoreDir(...segments: string[]): string {
  const nested = path.join(pymesRepoRoot, 'core', ...segments);
  const sibling = path.join(pymesParentDir, 'core', ...segments);
  if (fs.existsSync(nested)) {
    return nested;
  }
  if (fs.existsSync(sibling)) {
    return sibling;
  }
  return sibling;
}

/**
 * TS bajo `modules/`: repo hermano `../../modules/...` (mismo padre que `pymes/`).
 */
function monorepoModulesDir(...segments: string[]): string {
  return path.join(pymesParentDir, 'modules', ...segments);
}

/**
 * Checkout hermano `modules/` o paquete publicado en node_modules.
 */
function monorepoModulesDirOrNodeModule(segments: string[], nodeModulesSpecifier: string): string {
  const sibling = path.join(pymesParentDir, 'modules', ...segments);
  if (fs.existsSync(sibling)) {
    return sibling;
  }
  return fileURLToPath(new URL(nodeModulesSpecifier, import.meta.url));
}

/**
 * Paquetes publicados en npm: preferir `node_modules` para alinear con `tsc` y CI (checkout de `modules` puede ir detrás de main).
 * Si no hay paquete instalado, seguir usando el monorepo local.
 */
function modulesPackagePreferNodeModules(segments: string[], nodeModulesSpecifier: string): string {
  const published = fileURLToPath(new URL(nodeModulesSpecifier, import.meta.url));
  if (fs.existsSync(published)) {
    return published;
  }
  return monorepoModulesDir(...segments);
}

/** `surface.ts` (≥0.7.0) o `crudCanonicalSurface.tsx` (0.6.x) en npm, si no monorepo local. */
function modulesCrudUiSurfacePath(): string {
  const surfaceNew = fileURLToPath(new URL('./node_modules/@devpablocristo/modules-crud-ui/src/surface.ts', import.meta.url));
  const surfaceLegacy = fileURLToPath(
    new URL('./node_modules/@devpablocristo/modules-crud-ui/src/crudCanonicalSurface.tsx', import.meta.url),
  );
  if (fs.existsSync(surfaceNew)) return surfaceNew;
  if (fs.existsSync(surfaceLegacy)) return surfaceLegacy;
  return monorepoModulesDir('crud/ui/ts/src/surface.ts');
}

const fullCalendarCore = fileURLToPath(new URL('./node_modules/@fullcalendar/core', import.meta.url));
const fullCalendarDayGrid = fileURLToPath(new URL('./node_modules/@fullcalendar/daygrid', import.meta.url));
const fullCalendarInteraction = fileURLToPath(new URL('./node_modules/@fullcalendar/interaction', import.meta.url));
const fullCalendarReact = fileURLToPath(new URL('./node_modules/@fullcalendar/react', import.meta.url));
const fullCalendarTimeGrid = fileURLToPath(new URL('./node_modules/@fullcalendar/timegrid', import.meta.url));
const fullCalendarList = fileURLToPath(new URL('./node_modules/@fullcalendar/list', import.meta.url));
const tanstackReactQuery = fileURLToPath(new URL('./node_modules/@tanstack/react-query', import.meta.url));
const coreBrowserIndex = monorepoCoreDir('browser/ts/src/index.ts');
const coreBrowserCrud = monorepoCoreDir('browser/ts/src/crud/index.ts');
const coreBrowserSearch = monorepoCoreDir('browser/ts/src/search/index.ts');
const coreAuthnErrors = monorepoCoreDir('authn/ts/src/errors.ts');
const coreFsmIndex = monorepoCoreDir('concurrency/fsm/ts/src/index.ts');
const coreBrowserStorage = monorepoCoreDir('browser/ts/src/storage.ts');
const coreBrowserTheme = monorepoCoreDir('browser/ts/src/theme.ts');
const coreBrowserObservability = monorepoCoreDir('browser/ts/src/observability.ts');
const coreBrowserI18n = monorepoCoreDir('browser/ts/src/i18n/index.ts');
const modulesCrudUiIndex = monorepoModulesDirOrNodeModule(
  ['crud/ui/ts/src/index.ts'],
  './node_modules/@devpablocristo/modules-crud-ui/src/index.ts',
);
const modulesCrudUiCsv = monorepoModulesDirOrNodeModule(
  ['crud/ui/ts/src/csv.ts'],
  './node_modules/@devpablocristo/modules-crud-ui/src/csv.ts',
);
const modulesCrudUiSurface = modulesCrudUiSurfacePath();
/** Repo hermano `modules/`; si no existe en disco, fallback a paquete en node_modules (CI con symlink). */
const modulesCalendarBoardIndex = monorepoModulesDirOrNodeModule(
  ['calendar/board/ts/src/index.ts'],
  './node_modules/@devpablocristo/modules-calendar-board/src/index.ts',
);
const modulesCalendarBoardStyles = monorepoModulesDirOrNodeModule(
  ['calendar/board/ts/src/styles.css'],
  './node_modules/@devpablocristo/modules-calendar-board/src/styles.css',
);
const modulesKanbanBoardIndex = modulesPackagePreferNodeModules(
  ['kanban/board/ts/src/index.ts'],
  './node_modules/@devpablocristo/modules-kanban-board/src/index.ts',
);
const modulesSchedulingIndex = monorepoModulesDir('scheduling/ts/src/index.ts');
const modulesSchedulingNext = monorepoModulesDir('scheduling/ts/src/next.ts');
const modulesSchedulingStyles = monorepoModulesDir('scheduling/ts/src/styles.css');
const modulesSchedulingStylesNext = monorepoModulesDir('scheduling/ts/src/styles.next.css');
const modulesWorkOrdersIndex = monorepoModulesDir('work-orders/ts/src/index.ts');
const modulesWorkOrdersStyles = monorepoModulesDir('work-orders/ts/src/styles.css');
const modulesShellSidebarIndex = monorepoModulesDir('sidebar/ts/src/index.ts');
const modulesShellSidebarStyles = monorepoModulesDir('sidebar/ts/src/styles.css');
const modulesUiModalStyles = monorepoModulesDir('ui/modal/ts/src/styles.css');
const modulesUiPageShellIndex = monorepoModulesDir('ui/page-shell/ts/src/index.ts');
const modulesUiPageShellStyles = monorepoModulesDir('ui/page-shell/ts/src/styles.css');
const modulesUiNotificationFeedIndex = monorepoModulesDirOrNodeModule(
  ['ui/notification-feed/ts/src/index.ts'],
  './node_modules/@devpablocristo/modules-ui-notification-feed/src/index.ts',
);
const modulesUiNotificationFeedStyles = monorepoModulesDirOrNodeModule(
  ['ui/notification-feed/ts/src/styles.css'],
  './node_modules/@devpablocristo/modules-ui-notification-feed/src/styles.css',
);
const modulesUiSectionHubIndex = monorepoModulesDir('ui/section-hub/ts/src/index.tsx');
const modulesUiSectionHubStyles = monorepoModulesDir('ui/section-hub/ts/src/styles.css');

export default defineConfig({
  envDir: '..',
  cacheDir: process.env.VITE_CACHE_DIR ?? 'node_modules/.vite',
  plugins: [react()],
  resolve: {
    preserveSymlinks: true,
    alias: [
      { find: '@fullcalendar/core', replacement: fullCalendarCore },
      { find: '@fullcalendar/daygrid', replacement: fullCalendarDayGrid },
      { find: '@fullcalendar/interaction', replacement: fullCalendarInteraction },
      { find: '@fullcalendar/react', replacement: fullCalendarReact },
      { find: '@fullcalendar/timegrid', replacement: fullCalendarTimeGrid },
      { find: '@fullcalendar/list', replacement: fullCalendarList },
      { find: '@tanstack/react-query', replacement: tanstackReactQuery },
      { find: '@devpablocristo/core-authn/errors', replacement: coreAuthnErrors },
      { find: '@devpablocristo/core-browser/crud', replacement: coreBrowserCrud },
      { find: '@devpablocristo/core-browser/search', replacement: coreBrowserSearch },
      { find: '@devpablocristo/core-browser/storage', replacement: coreBrowserStorage },
      { find: '@devpablocristo/core-browser/theme', replacement: coreBrowserTheme },
      { find: '@devpablocristo/core-browser/observability', replacement: coreBrowserObservability },
      { find: '@devpablocristo/core-browser/i18n', replacement: coreBrowserI18n },
      { find: '@devpablocristo/core-fsm', replacement: coreFsmIndex },
      { find: '@devpablocristo/core-browser', replacement: coreBrowserIndex },
      { find: '@devpablocristo/modules-calendar-board/styles.css', replacement: modulesCalendarBoardStyles },
      { find: '@devpablocristo/modules-calendar-board', replacement: modulesCalendarBoardIndex },
      { find: '@devpablocristo/modules-crud-ui/csv', replacement: modulesCrudUiCsv },
      { find: '@devpablocristo/modules-crud-ui/surface', replacement: modulesCrudUiSurface },
      { find: '@devpablocristo/modules-crud-ui', replacement: modulesCrudUiIndex },
      { find: '@devpablocristo/modules-kanban-board', replacement: modulesKanbanBoardIndex },
      { find: /^@devpablocristo\/modules-scheduling\/styles\.next\.css$/, replacement: modulesSchedulingStylesNext },
      { find: /^@devpablocristo\/modules-scheduling\/styles\.css$/, replacement: modulesSchedulingStyles },
      { find: /^@devpablocristo\/modules-scheduling\/next$/, replacement: modulesSchedulingNext },
      { find: /^@devpablocristo\/modules-scheduling$/, replacement: modulesSchedulingIndex },
      { find: '@devpablocristo/modules-work-orders/styles.css', replacement: modulesWorkOrdersStyles },
      { find: '@devpablocristo/modules-work-orders', replacement: modulesWorkOrdersIndex },
      { find: '@devpablocristo/modules-shell-sidebar/styles.css', replacement: modulesShellSidebarStyles },
      { find: '@devpablocristo/modules-shell-sidebar', replacement: modulesShellSidebarIndex },
      { find: '@devpablocristo/modules-ui-modal/styles.css', replacement: modulesUiModalStyles },
      { find: '@devpablocristo/modules-ui-page-shell/styles.css', replacement: modulesUiPageShellStyles },
      { find: '@devpablocristo/modules-ui-page-shell', replacement: modulesUiPageShellIndex },
      { find: '@devpablocristo/modules-ui-notification-feed/styles.css', replacement: modulesUiNotificationFeedStyles },
      { find: '@devpablocristo/modules-ui-notification-feed', replacement: modulesUiNotificationFeedIndex },
      { find: '@devpablocristo/modules-ui-section-hub/styles.css', replacement: modulesUiSectionHubStyles },
      { find: '@devpablocristo/modules-ui-section-hub', replacement: modulesUiSectionHubIndex },
    ],
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (id.includes('@clerk/')) {
              return 'vendor-clerk';
            }
            if (id.includes('react-router')) {
              return 'vendor-router';
            }
            if (id.includes('@fullcalendar')) {
              return 'vendor-calendar';
            }
            if (id.includes('@tanstack/react-query')) {
              return 'vendor-query';
            }
            if (id.includes('@devpablocristo/modules-kanban-board') || id.includes('@dnd-kit/')) {
              return 'vendor-kanban';
            }
            if (id.includes('@devpablocristo/modules-crud-ui')) {
              return 'vendor-crud';
            }
            if (id.includes('@devpablocristo/core-authn')) {
              return 'vendor-authn';
            }
            if (id.includes('@devpablocristo/core-browser') || id.includes('@devpablocristo/core-http')) {
              return 'vendor-core';
            }
            return undefined;
          }
          return undefined;
        },
      },
    },
  },
  server: {
    port: 5173,
    host: '0.0.0.0',
    fs: {
      allow: [searchForWorkspaceRoot(process.cwd())],
    },
    watch: {
      usePolling: true,
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    exclude: [...configDefaults.exclude, '**/e2e/**'],
  },
});
