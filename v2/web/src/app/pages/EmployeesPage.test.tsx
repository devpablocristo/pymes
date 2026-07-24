import type { HttpClient } from "@devpablocristo/platform-http";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import type { AuthContextValue } from "../../auth/AuthContext";
import { AppProviders } from "../providers/AppProviders";
import { EmployeesPage } from "./EmployeesPage";

const authValue: AuthContextValue = {
  status: "signed-in",
  productRole: "owner",
  activeOrganizationId: "tenant-1",
  organizations: [
    {
      id: "tenant-1",
      switchKey: "clerk-tenant-1",
      name: "Pymes Base",
      role: "owner",
    },
  ],
  user: { id: "user-1", displayName: "Pablo Cristo" },
  getToken: vi.fn(async () => "token"),
  setActiveOrganization: vi.fn(async () => undefined),
  signOut: vi.fn(async () => undefined),
};

test("uses row selection and one bulk lifecycle toolbar", async () => {
  const user = userEvent.setup();
  const onLifecycle = vi.fn(async () => undefined);
  const apiClient = {
    request: vi.fn(),
    requestResponse: vi.fn(),
  } as unknown as HttpClient;

  render(
    <AppProviders authValue={authValue} apiClient={apiClient}>
      <EmployeesPage
        employees={[
          {
            id: "employee-1",
            first_name: "Marina",
            last_name: "Sur",
            status: "active",
          },
        ]}
        onLifecycle={onLifecycle}
      />
    </AppProviders>,
  );

  expect(
    screen.queryByRole("columnheader", { name: "Acciones" }),
  ).not.toBeInTheDocument();
  await user.click(screen.getByRole("checkbox", { name: "Seleccionar Marina Sur" }));
  await user.click(screen.getByRole("button", { name: "Archivar" }));

  expect(onLifecycle).toHaveBeenCalledWith(
    expect.objectContaining({ id: "employee-1" }),
    "archive",
  );

  await user.click(screen.getByRole("tab", { name: "Archivados" }));
  expect(screen.getByText("Marina Sur")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Desarchivar" })).toBeDisabled();
});
