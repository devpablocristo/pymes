import { afterEach, describe, expect, it, vi } from "vitest";
import { createHttpSchedulingGateway } from "./httpSchedulingGateway";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("HTTP Scheduling Gateway", () => {
  it("falla cerrado antes del transporte cuando Clerk no entrega token", async () => {
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    const gateway = createHttpSchedulingGateway("https://api.example.test");

    await expect(
      gateway.listBranches({
        organizationId: "org_local",
        getToken: async () => null,
      }),
    ).rejects.toMatchObject({
      code: "FORBIDDEN",
      status: 401,
    });
    expect(fetch).not.toHaveBeenCalled();
  });
});
