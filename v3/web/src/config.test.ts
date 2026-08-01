import { describe, expect, it, vi } from "vitest";

describe("configuración web", () => {
  it("mantiene los fakes fuera de los bundles productivos", async () => {
    vi.stubEnv("PROD", true);
    vi.stubEnv("MODE", "production");
    vi.stubEnv("VITE_USE_FAKE_API", "true");
    vi.resetModules();
    const { loadWebConfig } = await import("./config");
    expect(() => loadWebConfig()).toThrow(/VITE_USE_FAKE_API/);
    vi.unstubAllEnvs();
  });
});
