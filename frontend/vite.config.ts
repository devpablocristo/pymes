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
 * Resuelve `core/...` o `modules/...` desde layout anidado (`pymes/modules`) o hermano (`../modules`).
 * CI puede clonar dentro del workspace; desarrollo local suele tener clones junto al repo.
 */
function monorepoPackageDir(kind: 'core' | 'modules', ...segments: string[]): string {
  const nested = path.join(pymesRepoRoot, kind, ...segments);
  const sibling = path.join(pymesParentDir, kind, ...segments);
  if (fs.existsSync(nested)) {
    return nested;
  }
  if (fs.existsSync(sibling)) {
    return sibling;
  }
  return sibling;
}

/**
 * Igual que monorepoPackageDir pero si no hay checkout local, usa el paquete publicado en node_modules.
 * Evita alias a rutas inexistentes cuando solo se usa npm (sin carpeta `pymes/modules`).
 */
function monorepoPackageDirOrNodeModule(
  kind: 'core' | 'modules',
  segments: string[],
  nodeModulesSpecifier: string,
): string {
  const nested = path.join(pymesRepoRoot, kind, ...segments);
  const sibling = path.join(pymesParentDir, kind, ...segments);
  if (fs.existsSync(nested)) {
    return nested;
  }
  if (fs.existsSync(sibling)) {
    return sibling;
  }
  return fileURLToPath(new URL(nodeModulesSpecifier, import.meta.url));
}

const fullCalendarCore = fileURLToPath(new URL('./node_modules/@fullcalendar/core', import.meta.url));
const fullCalendarDayGrid = fileURLToPath(new URL('./node_modules/@fullcalendar/daygrid', import.meta.url));
const fullCalendarInteraction = fileURLToPath(new URL('./node_modules/@fullcalendar/interaction', import.meta.url));
const fullCalendarReact = fileURLToPath(new URL('./node_modules/@fullcalendar/react', import.meta.url));
const fullCalendarTimeGrid = fileURLToPath(new URL('./node_modules/@fullcalendar/timegrid', import.meta.url));
const fullCalendarList = fileURLToPath(new URL('./node_modules/@fullcalendar/list', import.meta.url));
const tanstackReactQuery = fileURLToPath(new URL('./node_modules/@tanstack/react-query', import.meta.url));
const coreBrowserIndex = monorepoPackageDir('core', 'browser/ts/src/index.ts');
const coreBrowserCrud = monorepoPackageDir('core', 'browser/ts/src/crud/index.ts');
const coreBrowserSearch = monorepoPackageDir('core', 'browser/ts/src/search/index.ts');
const coreAuthnErrors = monorepoPackageDir('core', 'authn/ts/src/errors.ts');
const coreFsmIndex = monorepoPackageDir('core', 'concurrency/fsm/ts/src/index.ts');
const coreBrowserStorage = monorepoPackageDir('core', 'browser/ts/src/storage.ts');
const coreBrowserTheme = monorepoPackageDir('core', 'browser/ts/src/theme.ts');
const coreBrowserObservability = monorepoPackageDir('core', 'browser/ts/src/observability.ts');
const coreBrowserI18n = monorepoPackageDir('core', 'browser/ts/src/i18n/index.ts');
const modulesCrudUiIndex = monorepoPackageDirOrNodeModule(
  'modules',
  ['crud/ui/ts/src/index.ts'],
  './node_modules/@devpablocristo/modules-crud-ui/src/index.ts',
);
const modulesCrudUiCsv = monorepoPackageDirOrNodeModule(
  'modules',
  ['crud/ui/ts/src/csv.ts'],
  './node_modules/@devpablocristo/modules-crud-ui/src/csv.ts',
);
const modulesCrudUiSurface = monorepoPackageDirOrNodeModule(
  'modules',
  ['crud/ui/ts/src/crudCanonicalSurface.tsx'],
  './node_modules/@devpablocristo/modules-crud-ui/src/crudCanonicalSurface.tsx',
);
const modulesCalendarBoardIndex = monorepoPackageDir('modules', 'calendar/board/ts/src/index.ts');
const modulesCalendarBoardStyles = monorepoPackageDir('modules', 'calendar/board/ts/src/styles.css');
const modulesKanbanBoardIndex = monorepoPackageDir('modules', 'kanban/board/ts/src/index.ts');
const modulesSchedulingIndex = monorepoPackageDir('modules', 'scheduling/ts/src/index.ts');
const modulesSchedulingNext = monorepoPackageDir('modules', 'scheduling/ts/src/next.ts');
const modulesSchedulingStyles = monorepoPackageDir('modules', 'scheduling/ts/src/styles.css');
const modulesSchedulingStylesNext = monorepoPackageDir('modules', 'scheduling/ts/src/styles.next.css');
const modulesWorkOrdersIndex = monorepoPackageDir('modules', 'work-orders/ts/src/index.ts');
const modulesWorkOrdersStyles = monorepoPackageDir('modules', 'work-orders/ts/src/styles.css');
const modulesShellSidebarIndex = monorepoPackageDir('modules', 'sidebar/ts/src/index.ts');
const modulesShellSidebarStyles = monorepoPackageDir('modules', 'sidebar/ts/src/styles.css');
const modulesUiModalStyles = monorepoPackageDir('modules', 'ui/modal/ts/src/styles.css');
const modulesUiPageShellIndex = monorepoPackageDir('modules', 'ui/page-shell/ts/src/index.ts');
const modulesUiPageShellStyles = monorepoPackageDir('modules', 'ui/page-shell/ts/src/styles.css');
const modulesUiSectionHubIndex = monorepoPackageDir('modules', 'ui/section-hub/ts/src/index.tsx');
const modulesUiSectionHubStyles = monorepoPackageDir('modules', 'ui/section-hub/ts/src/styles.css');

export default defineConfig({
  envDir: '..',
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
