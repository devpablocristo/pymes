import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  envDir: '../..',
  plugins: [react()],
  resolve: {
    alias: {
      '@pymes/ts-pkg': path.resolve(__dirname, '../../pkgs/ts-pkg/src'),
      '@pymes/frontend-shared': path.resolve(__dirname, '../../control-plane/frontend/src/shared'),
    },
  },
  server: {
    port: 5174,
    host: '0.0.0.0',
    fs: {
      allow: [path.resolve(__dirname, '../..')],
    },
    watch: {
      usePolling: true,
    },
  },
});
