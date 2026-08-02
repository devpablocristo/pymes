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

  it("hace que el bundle E2E sea autocontenido y determinístico", async () => {
    vi.stubEnv("PROD", true);
    vi.stubEnv("MODE", "e2e");
    vi.resetModules();
    const { loadWebConfig } = await import("./config");

    expect(loadWebConfig()).toMatchObject({
      allowInsecureLocalAuth: true,
      useFakeApi: true,
      localOrganizationId: "org_e2e",
      publicOrganizationSlug: "centro-norte",
    });
    vi.unstubAllEnvs();
  });
});
