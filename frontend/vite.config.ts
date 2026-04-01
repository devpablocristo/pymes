import { fileURLToPath } from 'node:url';
import { defineConfig, searchForWorkspaceRoot } from 'vite';
import { configDefaults } from 'vitest/config';
import react from '@vitejs/plugin-react';

const fullCalendarCore = fileURLToPath(new URL('./node_modules/@fullcalendar/core', import.meta.url));
const fullCalendarDayGrid = fileURLToPath(new URL('./node_modules/@fullcalendar/daygrid', import.meta.url));
const fullCalendarInteraction = fileURLToPath(new URL('./node_modules/@fullcalendar/interaction', import.meta.url));
const fullCalendarReact = fileURLToPath(new URL('./node_modules/@fullcalendar/react', import.meta.url));
const fullCalendarTimeGrid = fileURLToPath(new URL('./node_modules/@fullcalendar/timegrid', import.meta.url));
const coreWorkspaceRoot = fileURLToPath(new URL('../../core', import.meta.url));
const modulesWorkspaceRoot = fileURLToPath(new URL('../../modules', import.meta.url));
const sidebarModule = fileURLToPath(new URL('../../modules/sidebar/ts/src/index.ts', import.meta.url));
const sidebarStyles = fileURLToPath(new URL('../../modules/sidebar/ts/src/styles.css', import.meta.url));
const coreBrowserTrigram = fileURLToPath(new URL('../../core/browser/ts/src/trigram.ts', import.meta.url));
const notificationFeedModule = fileURLToPath(
  new URL('../../modules/ui/notification-feed/ts/src/index.ts', import.meta.url),
);
const notificationFeedStyles = fileURLToPath(
  new URL('../../modules/ui/notification-feed/ts/src/styles.css', import.meta.url),
);

export default defineConfig({
  envDir: '..',
  plugins: [react()],
  resolve: {
    alias: [
      {
        find: '@devpablocristo/modules-ui-notification-feed/styles.css',
        replacement: notificationFeedStyles,
      },
      { find: '@devpablocristo/modules-shell-sidebar/styles.css', replacement: sidebarStyles },
      { find: '@devpablocristo/modules-shell-sidebar', replacement: sidebarModule },
      { find: '@devpablocristo/core-browser/trigram', replacement: coreBrowserTrigram },
      { find: '@devpablocristo/modules-ui-notification-feed', replacement: notificationFeedModule },
      { find: '@fullcalendar/core', replacement: fullCalendarCore },
      { find: '@fullcalendar/daygrid', replacement: fullCalendarDayGrid },
      { find: '@fullcalendar/interaction', replacement: fullCalendarInteraction },
      { find: '@fullcalendar/react', replacement: fullCalendarReact },
      { find: '@fullcalendar/timegrid', replacement: fullCalendarTimeGrid },
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
      allow: [searchForWorkspaceRoot(process.cwd()), modulesWorkspaceRoot, coreWorkspaceRoot],
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
