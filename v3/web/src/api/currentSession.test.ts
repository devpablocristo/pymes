import { afterEach, describe, expect, it, vi } from "vitest";
import { getCurrentSession } from "./currentSession";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("sesión canónica", () => {
  it("usa el tenant local devuelto por el BFF, no el identificador de Clerk", async () => {
    const request = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      expect(String(input)).toBe("https://api.example.test/api/v1/session");
      expect(new Headers(init?.headers).get("Authorization")).toBe(
        "Bearer clerk-session-token",
      );
      expect(init?.cache).toBe("no-store");
      return new Response(
        JSON.stringify({
          actor_id: "user_clerk",
          organization: {
            id: "org_local",
            name: "Centro Norte",
            slug: "centro-norte",
            status: "ready",
          },
          role: "admin",
          permissions: ["scheduling:read"],
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      );
    });
    vi.stubGlobal("fetch", request);

    const session = await getCurrentSession(
      "https://api.example.test/",
      async () => "clerk-session-token",
    );

    expect(session.organization.id).toBe("org_local");
    expect(session.organization.id).not.toBe("org_clerk");
    expect(request).toHaveBeenCalledOnce();
  });

  it("falla cerrado sin token o sin membresía local", async () => {
    await expect(
      getCurrentSession("https://api.example.test", async () => null),
    ).rejects.toThrow(/token/);

    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(
          JSON.stringify({
            code: "FORBIDDEN",
            message: "La sesión no pertenece a una organización activa.",
          }),
          {
            status: 403,
            headers: { "Content-Type": "application/json" },
          },
        ),
      ),
    );
    await expect(
      getCurrentSession(
        "https://api.example.test",
        async () => "clerk-session-token",
      ),
    ).rejects.toThrow(/organización activa/);
  });
});
