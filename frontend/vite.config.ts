import path from 'node:path';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const coreAuthCandidates = [
  path.resolve(__dirname, '.deps/core/authn/ts/src'),
  path.resolve(__dirname, '../../core/authn/ts/src'),
];
const coreAuthPath = coreAuthCandidates.find((candidate) => fs.existsSync(candidate)) ?? coreAuthCandidates[0];

export default defineConfig({
  envDir: '..',
  plugins: [react()],
  resolve: {
    alias: {
      '@devpablocristo/core-authn': coreAuthPath,
    },
  },
  server: {
    port: 5173,
    host: '0.0.0.0',
    fs: {
      allow: [path.resolve(__dirname, '..'), coreAuthPath],
    },
    watch: {
      usePolling: true,
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
  },
});
