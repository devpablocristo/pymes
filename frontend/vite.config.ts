import { fileURLToPath } from 'node:url';
import { defineConfig, searchForWorkspaceRoot } from 'vite';
import { configDefaults } from 'vitest/config';
import react from '@vitejs/plugin-react';

const fullCalendarCore = fileURLToPath(new URL('./node_modules/@fullcalendar/core', import.meta.url));
const fullCalendarDayGrid = fileURLToPath(new URL('./node_modules/@fullcalendar/daygrid', import.meta.url));
const fullCalendarInteraction = fileURLToPath(new URL('./node_modules/@fullcalendar/interaction', import.meta.url));
const fullCalendarReact = fileURLToPath(new URL('./node_modules/@fullcalendar/react', import.meta.url));
const fullCalendarTimeGrid = fileURLToPath(new URL('./node_modules/@fullcalendar/timegrid', import.meta.url));
const coreBrowserIndex = fileURLToPath(new URL('../../core/browser/ts/src/index.ts', import.meta.url));
const coreBrowserCrud = fileURLToPath(new URL('../../core/browser/ts/src/crud/index.ts', import.meta.url));
const coreBrowserSearch = fileURLToPath(new URL('../../core/browser/ts/src/search/index.ts', import.meta.url));
const coreBrowserStorage = fileURLToPath(new URL('../../core/browser/ts/src/storage.ts', import.meta.url));
const modulesCrudUiIndex = fileURLToPath(new URL('../../modules/crud/ui/ts/src/index.ts', import.meta.url));
const modulesCrudUiCsv = fileURLToPath(new URL('../../modules/crud/ui/ts/src/csv.ts', import.meta.url));
const modulesKanbanBoardIndex = fileURLToPath(new URL('../../modules/kanban/board/ts/src/index.ts', import.meta.url));
const modulesShellSidebarIndex = fileURLToPath(new URL('../../modules/sidebar/ts/src/index.ts', import.meta.url));
const modulesShellSidebarStyles = fileURLToPath(new URL('../../modules/sidebar/ts/src/styles.css', import.meta.url));

export default defineConfig({
  envDir: '..',
  plugins: [react()],
  resolve: {
    alias: [
      { find: '@fullcalendar/core', replacement: fullCalendarCore },
      { find: '@fullcalendar/daygrid', replacement: fullCalendarDayGrid },
      { find: '@fullcalendar/interaction', replacement: fullCalendarInteraction },
      { find: '@fullcalendar/react', replacement: fullCalendarReact },
      { find: '@fullcalendar/timegrid', replacement: fullCalendarTimeGrid },
      { find: '@devpablocristo/core-browser/crud', replacement: coreBrowserCrud },
      { find: '@devpablocristo/core-browser/search', replacement: coreBrowserSearch },
      { find: '@devpablocristo/core-browser/storage', replacement: coreBrowserStorage },
      { find: '@devpablocristo/core-browser', replacement: coreBrowserIndex },
      { find: '@devpablocristo/modules-crud-ui/csv', replacement: modulesCrudUiCsv },
      { find: '@devpablocristo/modules-crud-ui', replacement: modulesCrudUiIndex },
      { find: '@devpablocristo/modules-kanban-board', replacement: modulesKanbanBoardIndex },
      { find: '@devpablocristo/modules-shell-sidebar/styles.css', replacement: modulesShellSidebarStyles },
      { find: '@devpablocristo/modules-shell-sidebar', replacement: modulesShellSidebarIndex },
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
            if (id.includes('@devpablocristo/modules-kanban-board') || id.includes('@hello-pangea/dnd') || id.includes('@dnd-kit/')) {
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
