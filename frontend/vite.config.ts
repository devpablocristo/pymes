import { fileURLToPath } from 'node:url';
import { defineConfig, searchForWorkspaceRoot } from 'vite';
import { configDefaults } from 'vitest/config';
import react from '@vitejs/plugin-react';

const fullCalendarCore = fileURLToPath(new URL('./node_modules/@fullcalendar/core', import.meta.url));
const fullCalendarDayGrid = fileURLToPath(new URL('./node_modules/@fullcalendar/daygrid', import.meta.url));
const fullCalendarInteraction = fileURLToPath(new URL('./node_modules/@fullcalendar/interaction', import.meta.url));
const fullCalendarReact = fileURLToPath(new URL('./node_modules/@fullcalendar/react', import.meta.url));
const fullCalendarTimeGrid = fileURLToPath(new URL('./node_modules/@fullcalendar/timegrid', import.meta.url));
const tanstackReactQuery = fileURLToPath(new URL('./node_modules/@tanstack/react-query', import.meta.url));
const coreBrowserIndex = fileURLToPath(new URL('../../core/browser/ts/src/index.ts', import.meta.url));
const coreBrowserCrud = fileURLToPath(new URL('../../core/browser/ts/src/crud/index.ts', import.meta.url));
const coreBrowserSearch = fileURLToPath(new URL('../../core/browser/ts/src/search/index.ts', import.meta.url));
const coreAuthnErrors = fileURLToPath(new URL('../../core/authn/ts/src/errors.ts', import.meta.url));
const coreBrowserStorage = fileURLToPath(new URL('../../core/browser/ts/src/storage.ts', import.meta.url));
const coreBrowserTheme = fileURLToPath(new URL('../../core/browser/ts/src/theme.ts', import.meta.url));
const coreBrowserObservability = fileURLToPath(new URL('../../core/browser/ts/src/observability.ts', import.meta.url));
const coreBrowserI18n = fileURLToPath(new URL('../../core/browser/ts/src/i18n/index.ts', import.meta.url));
const modulesCrudUiIndex = fileURLToPath(new URL('../../modules/crud/ui/ts/src/index.ts', import.meta.url));
const modulesCrudUiCsv = fileURLToPath(new URL('../../modules/crud/ui/ts/src/csv.ts', import.meta.url));
const modulesCalendarBoardIndex = fileURLToPath(new URL('../../modules/calendar/board/ts/src/index.ts', import.meta.url));
const modulesCalendarBoardStyles = fileURLToPath(new URL('../../modules/calendar/board/ts/src/styles.css', import.meta.url));
const modulesKanbanBoardIndex = fileURLToPath(new URL('../../modules/kanban/board/ts/src/index.ts', import.meta.url));
const modulesSchedulingIndex = fileURLToPath(new URL('../../modules/scheduling/ts/src/index.ts', import.meta.url));
const modulesSchedulingStyles = fileURLToPath(new URL('../../modules/scheduling/ts/src/styles.css', import.meta.url));
const modulesShellSidebarIndex = fileURLToPath(new URL('../../modules/sidebar/ts/src/index.ts', import.meta.url));
const modulesShellSidebarStyles = fileURLToPath(new URL('../../modules/sidebar/ts/src/styles.css', import.meta.url));
const modulesUiModalStyles = fileURLToPath(new URL('../../modules/ui/modal/ts/src/styles.css', import.meta.url));
const modulesUiPageShellIndex = fileURLToPath(new URL('../../modules/ui/page-shell/ts/src/index.ts', import.meta.url));
const modulesUiPageShellStyles = fileURLToPath(new URL('../../modules/ui/page-shell/ts/src/styles.css', import.meta.url));
const modulesUiSectionHubIndex = fileURLToPath(new URL('../../modules/ui/section-hub/ts/src/index.tsx', import.meta.url));
const modulesUiSectionHubStyles = fileURLToPath(new URL('../../modules/ui/section-hub/ts/src/styles.css', import.meta.url));

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
      { find: '@tanstack/react-query', replacement: tanstackReactQuery },
      { find: '@devpablocristo/core-authn/errors', replacement: coreAuthnErrors },
      { find: '@devpablocristo/core-browser/crud', replacement: coreBrowserCrud },
      { find: '@devpablocristo/core-browser/search', replacement: coreBrowserSearch },
      { find: '@devpablocristo/core-browser/storage', replacement: coreBrowserStorage },
      { find: '@devpablocristo/core-browser/theme', replacement: coreBrowserTheme },
      { find: '@devpablocristo/core-browser/observability', replacement: coreBrowserObservability },
      { find: '@devpablocristo/core-browser/i18n', replacement: coreBrowserI18n },
      { find: '@devpablocristo/core-browser', replacement: coreBrowserIndex },
      { find: '@devpablocristo/modules-calendar-board/styles.css', replacement: modulesCalendarBoardStyles },
      { find: '@devpablocristo/modules-calendar-board', replacement: modulesCalendarBoardIndex },
      { find: '@devpablocristo/modules-crud-ui/csv', replacement: modulesCrudUiCsv },
      { find: '@devpablocristo/modules-crud-ui', replacement: modulesCrudUiIndex },
      { find: '@devpablocristo/modules-kanban-board', replacement: modulesKanbanBoardIndex },
      { find: /^@devpablocristo\/modules-scheduling\/styles\.css$/, replacement: modulesSchedulingStyles },
      { find: /^@devpablocristo\/modules-scheduling$/, replacement: modulesSchedulingIndex },
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
