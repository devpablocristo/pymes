import { defineConfig, searchForWorkspaceRoot } from 'vite';
import { configDefaults } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  envDir: '..',
  cacheDir: process.env.VITE_CACHE_DIR ?? 'node_modules/.vite',
  plugins: [react()],
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
