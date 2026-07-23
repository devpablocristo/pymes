import type { HttpClient } from "@devpablocristo/platform-http";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, test, vi } from "vitest";
import type { AuthContextValue } from "../auth/AuthContext";
import { App } from "./App";
import { AppProviders } from "./providers/AppProviders";

const activeOrganization = {
  id: "11111111-1111-4111-8111-111111111111",
  switchKey: "org_clerk_norte",
  name: "Comercio Norte",
  slug: "comercio-norte",
  role: "owner" as const,
};

function createAuthValue(
  overrides: Partial<AuthContextValue> = {},
): AuthContextValue {
  return {
    status: "signed-in",
    sessionId: "sess_current",
    activeOrganizationId: activeOrganization.id,
    organizations: [activeOrganization],
    user: {
      id: "user_clerk_01",
      email: "ana@example.test",
      displayName: "Ana Pérez",
    },
    getToken: vi.fn(async () => "jwt"),
    setActiveOrganization: vi.fn(async () => undefined),
    signOut: vi.fn(async () => undefined),
    ...overrides,
  };
}

function currentSession(
  role: "owner" | "admin" | "member",
  permissions: string[],
) {
  return {
    user: {
      id: "33333333-3333-4333-8333-333333333333",
      email: "ana@example.test",
      display_name: "Ana Pérez",
    },
    organization: {
      id: activeOrganization.id,
      name: activeOrganization.name,
      slug: activeOrganization.slug,
      status: "active",
      role,
      switch_key: activeOrganization.switchKey,
      sync_status: "synced",
    },
    membership: {
      id: "22222222-2222-4222-8222-222222222222",
      role,
      status: "active",
    },
    role,
    permissions,
    session_id: "sess_current",
  };
}

function renderSettings(
  path: string,
  authValue: AuthContextValue,
  request: (path: string, options?: unknown) => Promise<unknown>,
) {
  window.history.replaceState({}, "", path);
  const apiClient = {
    request: vi.fn(request),
    requestResponse: vi.fn(),
  } as unknown as HttpClient;

  render(
    <AppProviders authValue={authValue} apiClient={apiClient}>
      <App />
    </AppProviders>,
  );

  return apiClient.request as ReturnType<typeof vi.fn>;
}

beforeEach(() => {
  window.localStorage.clear();
  window.history.replaceState({}, "", "/");
});

test("members see the team but the web never requests invitation emails", async () => {
  const request = renderSettings(
    "/settings/team",
    createAuthValue(),
    async (path) => {
      if (path === "/api/v1/session") {
        return currentSession("member", [
          "organization:view",
          "team:view",
          "sessions:manage:self",
        ]);
      }
      if (path === "/api/v1/team/members?limit=100") {
        return {
          items: [
            {
              id: "22222222-2222-4222-8222-222222222222",
              user: {
                id: "33333333-3333-4333-8333-333333333333",
                email: "member@example.test",
                display_name: "Marina Sur",
              },
              role: "member",
              status: "active",
              sync_status: "synced",
            },
          ],
          page: { total: 1 },
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  expect(await screen.findByText("Marina Sur")).toBeInTheDocument();
  expect(screen.queryByText("Invitaciones")).not.toBeInTheDocument();
  expect(
    request.mock.calls.some(([path]) => String(path).includes("/team/invitations")),
  ).toBe(false);
});

test("admins with the effective permission can list invitation state", async () => {
  renderSettings(
    "/settings/team",
    createAuthValue(),
    async (path) => {
      if (path === "/api/v1/session") {
        return currentSession(
          "admin",
          [
            "organization:view",
            "team:view",
            "team:invitation:create",
            "team:invitation:manage",
          ],
        );
      }
      if (path === "/api/v1/team/members?limit=100") {
        return { items: [], page: { total: 0 } };
      }
      if (path === "/api/v1/team/invitations?limit=100") {
        return {
          items: [
            {
              id: "44444444-4444-4444-8444-444444444444",
              email: "invitee@example.test",
              role: "member",
              status: "pending",
              expires_at: "2026-08-01T12:00:00Z",
              sync_status: "queued",
            },
          ],
          page: { total: 1 },
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  expect(await screen.findByText("invitee@example.test")).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "Invitaciones" })).toBeInTheDocument();
});

test("owners invite an admin without sending a tenant identifier", async () => {
  const user = userEvent.setup();
  const request = renderSettings(
    "/settings/team",
    createAuthValue(),
    async (path, options) => {
      if (path === "/api/v1/session") {
        return currentSession("owner", [
          "organization:view",
          "organization:update",
          "team:view",
          "team:member:update",
          "team:member:remove",
          "team:invitation:create",
          "team:invitation:manage",
          "team:ownership:transfer",
        ]);
      }
      if (path === "/api/v1/team/members?limit=100") {
        return { items: [], page: { total: 0 } };
      }
      if (path === "/api/v1/team/invitations?limit=100") {
        return { items: [], page: { total: 0 } };
      }
      if (path === "/api/v1/team/invitations") {
        const requestOptions = options as {
          method?: string;
          headers?: Record<string, string>;
          body?: string;
        };
        expect(requestOptions.method).toBe("POST");
        expect(requestOptions.headers?.["Idempotency-Key"]).toBeTruthy();
        expect(JSON.parse(requestOptions.body ?? "{}")).toEqual({
          email: "new-admin@example.test",
          role: "admin",
        });
        expect(requestOptions.body).not.toContain("org_id");
        return {};
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.type(
    await screen.findByRole("textbox", { name: "Email de la invitación" }),
    "New-Admin@Example.Test",
  );
  await user.selectOptions(
    screen.getByRole("combobox", { name: "Rol previsto" }),
    "admin",
  );
  await user.click(screen.getByRole("button", { name: "Enviar invitación" }));

  await waitFor(() =>
    expect(
      request.mock.calls.some(([path]) => path === "/api/v1/team/invitations"),
    ).toBe(true),
  );
});

test("revoking the current session uses idempotency and signs out locally", async () => {
  const user = userEvent.setup();
  const signOut = vi.fn(async () => undefined);
  const request = renderSettings(
    "/settings/sessions",
    createAuthValue({
      activeOrganizationId: undefined,
      organizations: [],
      signOut,
    }),
    async (path, options) => {
      if (path === "/api/v1/sessions?limit=100") {
        return {
          items: [
            {
              id: "sess_current",
              status: "active",
              created_at: "2026-07-20T12:00:00Z",
              last_active_at: "2026-07-23T12:00:00Z",
              expires_at: "2026-07-23T13:00:00Z",
              current: true,
            },
          ],
          page: { total: 1 },
        };
      }
      if (path === "/api/v1/sessions/sess_current") {
        const requestOptions = options as {
          method?: string;
          headers?: Record<string, string>;
        };
        expect(requestOptions.method).toBe("DELETE");
        expect(requestOptions.headers?.["Idempotency-Key"]).toBeTruthy();
        return undefined;
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.click(await screen.findByRole("button", { name: "Revocar" }));

  await waitFor(() => expect(signOut).toHaveBeenCalledOnce());
  expect(
    request.mock.calls.some(([path]) => path === "/api/v1/sessions/sess_current"),
  ).toBe(true);
});

test("updates the active organization without sending a tenant identifier", async () => {
  const user = userEvent.setup();
  const request = renderSettings(
    "/settings/organization",
    createAuthValue(),
    async (path, options) => {
      if (path === "/api/v1/session") {
        return {
          user: {
            id: "33333333-3333-4333-8333-333333333333",
            email: "owner@example.test",
            display_name: "Ana Pérez",
          },
          organization: {
            id: activeOrganization.id,
            name: "Comercio Norte",
            slug: "comercio-norte",
            status: "active",
            role: "owner",
            sync_status: "synced",
          },
          membership: {
            id: "22222222-2222-4222-8222-222222222222",
            role: "owner",
            status: "active",
          },
          role: "owner",
          permissions: ["organization:view", "organization:update"],
          session_id: "sess_current",
        };
      }
      if (path === "/api/v1/organization") {
        const requestOptions = options as {
          method?: string;
          headers?: Record<string, string>;
          body?: string;
        };
        expect(requestOptions.method).toBe("PATCH");
        expect(requestOptions.headers?.["Idempotency-Key"]).toBeTruthy();
        expect(JSON.parse(requestOptions.body ?? "{}")).toEqual({
          name: "Comercio Federal",
        });
        expect(requestOptions.body).not.toContain("org_id");
        return {
          id: activeOrganization.id,
          name: "Comercio Federal",
          slug: "comercio-norte",
          status: "active",
          role: "owner",
          sync_status: "queued",
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  const name = await screen.findByRole("textbox", { name: "Nombre" });
  await user.clear(name);
  await user.type(name, "Comercio Federal");
  await user.click(screen.getByRole("button", { name: "Guardar cambios" }));

  expect(await screen.findByRole("status")).toHaveTextContent("Cambio encolado y guardado.");
  expect(
    request.mock.calls.some(([path]) => path === "/api/v1/organization"),
  ).toBe(true);
});
