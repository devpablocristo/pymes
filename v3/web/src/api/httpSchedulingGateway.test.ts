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

  it("deriva una clave estable del command ID para reintentos idénticos", async () => {
    const fetch = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        new Response("[]", {
          status: 201,
          headers: { "Content-Type": "application/json" },
        }),
    );
    vi.stubGlobal("fetch", fetch);
    const gateway = createHttpSchedulingGateway("https://api.example.test");
    const identity = {
      organizationId: "org_local",
      getToken: async () => "clerk-token",
    };
    const input = {
      id: "11111111-1111-4111-8111-111111111111",
      branch_id: "22222222-2222-4222-8222-222222222222",
      service_id: "33333333-3333-4333-8333-333333333333",
      customer: { name: "Ada" },
      start_at: "2026-08-03T12:00:00Z",
      participants: 1,
    };

    await gateway.createBooking(identity, input);
    await gateway.createBooking(identity, input);

    const keys = fetch.mock.calls.map(([request, init]) =>
      new Headers(
        request instanceof Request ? request.headers : init?.headers,
      ).get("Idempotency-Key"),
    );
    expect(keys).toEqual([
      `booking:create:${input.id}`,
      `booking:create:${input.id}`,
    ]);
  });

  it("rechaza creates sin command ID antes del transporte", async () => {
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    const gateway = createHttpSchedulingGateway("https://api.example.test");

    await expect(
      gateway.createBooking(
        {
          organizationId: "org_local",
          getToken: async () => "clerk-token",
        },
        {
          branch_id: "22222222-2222-4222-8222-222222222222",
          service_id: "33333333-3333-4333-8333-333333333333",
          customer: { name: "Ada" },
          start_at: "2026-08-03T12:00:00Z",
          participants: 1,
        },
      ),
    ).rejects.toMatchObject({ code: "VALIDATION_ERROR" });
    expect(fetch).not.toHaveBeenCalled();
  });
});
