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

export default defineConfig({
  envDir: '..',
  plugins: [react()],
  resolve: {
    alias: {
      '@devpablocristo/core-authn': coreAuthPath,
      '@devpablocristo/core-browser': coreBrowserPath,
      '@devpablocristo/core-http': coreHttpPath,
    },
  },
  server: {
    port: 5173,
    host: '0.0.0.0',
    fs: {
      allow: [path.resolve(__dirname, '..'), coreAuthPath, coreBrowserPath, coreHttpPath],
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
