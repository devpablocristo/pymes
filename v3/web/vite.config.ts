import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig(({ mode }) => ({
  plugins: [react()],
  server: {
    host: "127.0.0.1",
    port: 4173,
    strictPort: true,
    proxy: {
      "/api": {
        target: process.env.PYMES_API_PROXY ?? "http://127.0.0.1:18080",
        changeOrigin: true,
      },
    },
  },
  preview: {
    host: "127.0.0.1",
    port: 4173,
    strictPort: true,
  },
  build: {
    target: "es2022",
    // Los mapas del bundle desplegable exponen el código fuente completo. Los
    // E2E locales pueden conservarlos, pero producción no los publica.
    sourcemap: mode !== "production",
  },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}"],
    setupFiles: "./src/test/setup.ts",
    // User-event and jsdom are CPU-bound. Keep the local/CI gate deterministic
    // and avoid multiplying browser-like workers on developer machines.
    fileParallelism: false,
    // A cold dependency install can spend more than Vitest's 5 s default
    // transforming FullCalendar before the first jsdom interaction settles.
    testTimeout: 15_000,
    css: true,
    coverage: {
      reporter: ["text", "json-summary"],
    },
  },
}));
