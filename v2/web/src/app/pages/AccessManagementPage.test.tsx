import type { HttpClient } from "@devpablocristo/platform-http";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import type { AuthContextValue } from "../../auth/AuthContext";
import { AppProviders } from "../providers/AppProviders";
import { AccessManagementPage } from "./AccessManagementPage";

const authValue: AuthContextValue = {
  status: "signed-in",
  productRole: "owner",
  activeOrganizationId: "tenant-1",
  organizations: [
    {
      id: "tenant-1",
      switchKey: "clerk-tenant-1",
      name: "Pymes Base",
      slug: "pymes-base",
      role: "owner",
    },
  ],
  user: {
    id: "user-1",
    email: "owner@example.test",
    displayName: "Pablo Cristo",
  },
  getToken: vi.fn(async () => "token"),
  setActiveOrganization: vi.fn(async () => undefined),
  signOut: vi.fn(async () => undefined),
};

test("filters users and tenants by active, archived and trash lifecycle states", async () => {
  const user = userEvent.setup();
  const request = vi.fn(async (path: string) => {
    if (path.startsWith("/api/v1/admin/users")) {
      return { items: [], page: { total: 0 } };
    }
    if (path.startsWith("/api/v1/admin/tenants")) {
      return { items: [], page: { total: 0 } };
    }
    if (path === "/api/v1/team/invitations?limit=100") {
      return { items: [], page: { total: 0 } };
    }
    throw new Error(`unexpected request ${path}`);
  });
  const apiClient = {
    request,
    requestResponse: vi.fn(),
  } as unknown as HttpClient;

  render(
    <AppProviders authValue={authValue} apiClient={apiClient}>
      <AccessManagementPage />
    </AppProviders>,
  );

  await waitFor(() => {
    expect(request).toHaveBeenCalledWith(
      "/api/v1/admin/users?limit=100&lifecycle_state=active",
      expect.anything(),
    );
  });

  await user.click(screen.getByRole("tab", { name: "Archivados" }));
  await waitFor(() => {
    expect(request).toHaveBeenCalledWith(
      "/api/v1/admin/users?limit=100&lifecycle_state=archived",
      expect.anything(),
    );
  });

  await user.click(screen.getByRole("tab", { name: "Tenants" }));
  await user.click(screen.getByRole("tab", { name: "Papelera" }));
  await waitFor(() => {
    expect(request).toHaveBeenCalledWith(
      "/api/v1/admin/tenants?limit=100&lifecycle_state=trashed",
      expect.anything(),
    );
  });
});

test("applies and reverses lifecycle transitions for users and tenants", async () => {
  const user = userEvent.setup();
  vi.spyOn(window, "confirm").mockReturnValue(true);
  const request = vi.fn(async (path: string, options?: { method?: string }) => {
    if (options?.method) return undefined;
    if (path.startsWith("/api/v1/admin/users")) {
      return {
        items: [
          {
            id: "user-2",
            email: "member@example.test",
            display_name: "Marina Sur",
            product_role: "user",
            memberships: [],
          },
        ],
        page: { total: 1 },
      };
    }
    if (path.startsWith("/api/v1/admin/tenants")) {
      return {
        items: [
          {
            id: "tenant-2",
            name: "Comercio Sur",
            slug: "comercio-sur",
            status: "active",
          },
        ],
        page: { total: 1 },
      };
    }
    if (path === "/api/v1/team/invitations?limit=100") {
      return { items: [], page: { total: 0 } };
    }
    throw new Error(`unexpected request ${path}`);
  });
  const apiClient = {
    request,
    requestResponse: vi.fn(),
  } as unknown as HttpClient;

  render(
    <AppProviders authValue={authValue} apiClient={apiClient}>
      <AccessManagementPage />
    </AppProviders>,
  );

  await user.click(await screen.findByRole("button", { name: "Archivar" }));
  expect(request).toHaveBeenCalledWith(
    "/api/v1/admin/users/user-2/archive",
    expect.objectContaining({ method: "POST" }),
  );

  await user.click(screen.getByRole("tab", { name: "Archivados" }));
  await user.click(await screen.findByRole("button", { name: "Desarchivar" }));
  expect(request).toHaveBeenCalledWith(
    "/api/v1/admin/users/user-2/unarchive",
    expect.objectContaining({ method: "POST" }),
  );

  await user.click(screen.getByRole("tab", { name: "Tenants" }));
  await user.click(await screen.findByRole("button", { name: "Papelera" }));
  expect(request).toHaveBeenCalledWith(
    "/api/v1/admin/tenants/tenant-2/trash",
    expect.objectContaining({ method: "POST" }),
  );

  await user.click(screen.getByRole("tab", { name: "Papelera" }));
  await user.click(await screen.findByRole("button", { name: "Restaurar" }));
  expect(request).toHaveBeenCalledWith(
    "/api/v1/admin/tenants/tenant-2/restore",
    expect.objectContaining({ method: "POST" }),
  );

  await user.click(screen.getByRole("button", { name: "Eliminar definitivamente" }));
  expect(request).toHaveBeenCalledWith(
    "/api/v1/admin/tenants/tenant-2/purge",
    expect.objectContaining({ method: "DELETE" }),
  );
});
