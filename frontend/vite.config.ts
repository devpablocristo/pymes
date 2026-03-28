import { defineConfig } from 'vite';
import { configDefaults } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  envDir: '..',
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
            return 'vendor';
          }
          if (id.includes('/src/crud/') || id.includes('/src/pages/ModulePage')) {
            return 'app-crud';
          }
          if (id.includes('/src/shared/frontendShell') || id.includes('/src/components/Shell') || id.includes('/src/pages/DashboardPage')) {
            return 'app-shell';
          }
          return undefined;
        },
      },
    },
  },
  server: {
    port: 5173,
    host: '0.0.0.0',
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
