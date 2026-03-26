import path from 'node:path';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite';
import { configDefaults } from 'vitest/config';
import react from '@vitejs/plugin-react';
import { wowdashAssetsPlugin } from './vite-wowdash-assets-plugin';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const coreAuthCandidates = [
  path.resolve(__dirname, '.deps/core/authn/ts/src'),
  path.resolve(__dirname, '../../core/authn/ts/src'),
];
const coreAuthPath = coreAuthCandidates.find((candidate) => fs.existsSync(candidate)) ?? coreAuthCandidates[0];
const coreBrowserCandidates = [
  path.resolve(__dirname, '.deps/core/browser/ts/src'),
  path.resolve(__dirname, '../../core/browser/ts/src'),
];
const coreBrowserPath = coreBrowserCandidates.find((candidate) => fs.existsSync(candidate)) ?? coreBrowserCandidates[0];
const coreHttpCandidates = [
  path.resolve(__dirname, '.deps/core/http/ts/src'),
  path.resolve(__dirname, '../../core/http/ts/src'),
];
const coreHttpPath = coreHttpCandidates.find((candidate) => fs.existsSync(candidate)) ?? coreHttpCandidates[0];
const modulesCrudTsCandidates = [
  path.resolve(__dirname, '.deps/modules/crud/ts'),
  path.resolve(__dirname, '../../modules/crud/ts'),
];
const modulesCrudTsRoot =
  modulesCrudTsCandidates.find((candidate) => fs.existsSync(candidate)) ?? modulesCrudTsCandidates[1];
const modulesCrudCrudSubpath = path.join(modulesCrudTsRoot, 'src', 'crud');
const modulesCrudKanbanSubpath = path.join(modulesCrudTsRoot, 'src', 'kanban');

export default defineConfig({
  envDir: '..',
  plugins: [react(), wowdashAssetsPlugin()],
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
            if (
              id.includes('apexcharts') ||
              id.includes('@fullcalendar') ||
              id.includes('datatables') ||
              id.includes('/jquery/') ||
              id.includes('react-bootstrap') ||
              id.includes('/bootstrap/')
            ) {
              return 'vendor-wowdash';
            }
            return 'vendor';
          }
          if (id.includes('/src/crud/') || id.includes('/src/pages/ModulePage')) {
            return 'app-crud';
          }
          if (id.includes('/src/shared/frontendShell') || id.includes('/src/components/Shell') || id.includes('/src/pages/DashboardPage')) {
            return 'app-shell';
          }
          if (id.includes('/src/wowdash-port/')) {
            return 'wowdash-template';
          }
          return undefined;
        },
      },
    },
  },
  resolve: {
    alias: {
      '#wowdash/App': path.resolve(__dirname, 'src/wowdash-port/App.jsx'),
      '@devpablocristo/core-authn': coreAuthPath,
      '@devpablocristo/core-browser': coreBrowserPath,
      '@devpablocristo/core-http': coreHttpPath,
      '@devpablocristo/modules-crud/crud': modulesCrudCrudSubpath,
      '@devpablocristo/modules-crud/kanban': modulesCrudKanbanSubpath,
      '@devpablocristo/modules-crud': path.join(modulesCrudTsRoot, 'src'),
    },
  },
  server: {
    port: 5173,
    host: '0.0.0.0',
    fs: {
      allow: [
        path.resolve(__dirname, '..'),
        path.resolve(__dirname, 'wowdash-assets'),
        coreAuthPath,
        coreBrowserPath,
        coreHttpPath,
        modulesCrudTsRoot,
        modulesCrudKanbanSubpath,
      ],
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
