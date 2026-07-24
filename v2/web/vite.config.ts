import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  cacheDir: "/tmp/pymes-vite-cache",
  plugins: [react()],
  server: {
    proxy: {
      "/api": process.env.VITE_PROXY_TARGET ?? "http://127.0.0.1:18080",
      "/webhooks": process.env.VITE_PROXY_TARGET ?? "http://127.0.0.1:18080",
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: "./src/test/setup.ts",
  },
});
