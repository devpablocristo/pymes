import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { beforeEach, expect, test, vi } from "vitest";
import type { AuthContextValue } from "../auth/AuthContext";
import { App } from "./App";
import { AppProviders } from "./providers/AppProviders";

vi.mock("@clerk/react", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@clerk/react")>();
  return {
    ...actual,
    SignIn: ({ path }: { path: string }) => (
      <div data-testid="clerk-sign-in" data-path={path}>
        Acceso por email
      </div>
    ),
  };
});

const organizations = [
  {
    id: "org_norte",
    switchKey: "clerk_org_norte",
    name: "Comercio Norte",
    slug: "comercio-norte",
    role: "owner" as const,
  },
  {
    id: "org_sur",
    switchKey: "clerk_org_sur",
    name: "Comercio Sur",
    slug: "comercio-sur",
    role: "member" as const,
  },
];

function createAuthValue(overrides: Partial<AuthContextValue> = {}): AuthContextValue {
  return {
    status: "signed-in",
    sessionId: "sess_test",
    activeOrganizationId: organizations[0].id,
    organizations,
    user: {
      id: "user_test",
      email: "ana@example.test",
      displayName: "Ana Pérez",
    },
    getToken: vi.fn(async () => "test-token"),
    setActiveOrganization: vi.fn(async () => undefined),
    signOut: vi.fn(async () => undefined),
    ...overrides,
  };
}

function renderApp(initialAuth = createAuthValue()) {
  function AuthHarness() {
    const [activeOrganizationId, setActiveOrganizationId] = useState(
      initialAuth.activeOrganizationId,
    );
    const authValue: AuthContextValue = {
      ...initialAuth,
      activeOrganizationId,
      setActiveOrganization: async (organizationId) => {
        await initialAuth.setActiveOrganization(organizationId);
        setActiveOrganizationId(organizationId);
      },
    };

    return (
      <AppProviders authValue={authValue}>
        <App />
      </AppProviders>
    );
  }

  return render(
    <AuthHarness />,
  );
}

beforeEach(() => {
  window.localStorage.clear();
  window.history.replaceState({}, "", "/");
});

test("redirects home into the product console with one main landmark", async () => {
  const authValue = createAuthValue();
  renderApp(authValue);

  expect(await screen.findByRole("heading", { name: "Inicio" })).toBeInTheDocument();
  expect(window.location.pathname).toBe("/dashboard");
  expect(document.querySelectorAll("main")).toHaveLength(1);
  expect(screen.getByRole("link", { name: "Inicio" })).toHaveAttribute("aria-current", "page");
  expect(screen.getByText("Comercio Norte")).toBeInTheDocument();
  expect(screen.getByText("Ana Pérez")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Cerrar sesión" }));
  expect(authValue.signOut).toHaveBeenCalledOnce();
  expect(screen.queryByText("Clientes")).not.toBeInTheDocument();
});

test("redirects signed-out users to the private sign-in route", async () => {
  renderApp(
    createAuthValue({
      status: "signed-out",
      activeOrganizationId: undefined,
      organizations: [],
      user: undefined,
    }),
  );

  expect(
    await screen.findByRole("heading", {
      name: "La gestión diaria empieza con una identidad segura.",
    }),
  ).toBeInTheDocument();
  expect(window.location.pathname).toBe("/sign-in");
  expect(screen.getByTestId("clerk-sign-in")).toHaveAttribute("data-path", "/sign-in");
});

test("fails closed when authentication is not configured", async () => {
  window.history.replaceState({}, "", "/sign-in");
  renderApp(
    createAuthValue({
      status: "unconfigured",
      activeOrganizationId: undefined,
      organizations: [],
      user: undefined,
    }),
  );

  expect(await screen.findByText("AUTH_NOT_CONFIGURED")).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "Falta conectar Clerk" })).toBeInTheDocument();
  expect(screen.queryByTestId("clerk-sign-in")).not.toBeInTheDocument();
  expect(document.querySelectorAll("main")).toHaveLength(1);
});

test("sends a signed-in user without organizations to no access", async () => {
  renderApp(
    createAuthValue({
      activeOrganizationId: undefined,
      organizations: [],
    }),
  );

  expect(
    await screen.findByRole("heading", {
      name: "Todavía no tenés una organización disponible.",
    }),
  ).toBeInTheDocument();
  expect(window.location.pathname).toBe("/no-access");
});

test("automatically activates the only available organization", async () => {
  const setActiveOrganization = vi.fn(async () => undefined);
  renderApp(
    createAuthValue({
      activeOrganizationId: undefined,
      organizations: [organizations[0]],
      setActiveOrganization,
    }),
  );

  await waitFor(() => {
    expect(setActiveOrganization).toHaveBeenCalledOnce();
    expect(setActiveOrganization).toHaveBeenCalledWith("org_norte");
  });
  expect(await screen.findByRole("heading", { name: "Inicio" })).toBeInTheDocument();
  expect(window.location.pathname).toBe("/dashboard");
});

test("asks the user to choose when multiple organizations are available", async () => {
  const user = userEvent.setup();
  const setActiveOrganization = vi.fn(async () => undefined);
  renderApp(
    createAuthValue({
      activeOrganizationId: undefined,
      setActiveOrganization,
    }),
  );

  expect(
    await screen.findByRole("heading", { name: "¿Dónde vas a trabajar?" }),
  ).toBeInTheDocument();
  expect(setActiveOrganization).not.toHaveBeenCalled();

  await user.click(screen.getByRole("button", { name: /Comercio Sur/ }));

  expect(setActiveOrganization).toHaveBeenCalledWith("org_sur");
  expect(await screen.findByRole("heading", { name: "Inicio" })).toBeInTheDocument();
});

test("supports keyboard search and accessible desktop and mobile navigation", async () => {
  const user = userEvent.setup();
  renderApp();
  await screen.findByRole("heading", { name: "Inicio" });

  fireEvent.keyDown(document, { key: "k", ctrlKey: true });
  expect(screen.getByRole("searchbox", { name: "Buscar en Pymes…" })).toHaveFocus();

  await user.click(screen.getByRole("button", { name: "Contraer navegación" }));
  expect(screen.getByRole("button", { name: "Expandir navegación" })).toHaveAttribute(
    "aria-expanded",
    "false",
  );

  await user.click(screen.getByRole("button", { name: "Abrir navegación" }));
  expect(screen.getByRole("button", { name: "Cerrar navegación" })).toHaveAttribute(
    "aria-expanded",
    "true",
  );
  fireEvent.keyDown(document, { key: "Escape" });
  expect(screen.getByRole("button", { name: "Abrir navegación" })).toHaveFocus();
});

test("persists language and theme through platform-browser", async () => {
  const user = userEvent.setup();
  const firstRender = renderApp();
  await screen.findByRole("heading", { name: "Inicio" });

  await user.click(screen.getByRole("button", { name: "Usar tema oscuro" }));
  await user.click(screen.getByRole("button", { name: "Cambiar idioma" }));

  expect(document.documentElement).toHaveAttribute("data-theme", "dark");
  expect(document.documentElement).toHaveAttribute("lang", "en-US");
  expect(window.localStorage.getItem("pymes-v2:theme")).toBe("dark");
  expect(window.localStorage.getItem("pymes-v2:language")).toBe("en");

  firstRender.unmount();
  renderApp();

  expect(await screen.findByRole("heading", { name: "Home" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Use light theme" })).toBeInTheDocument();
});

test("renders a useful 404 inside the console", async () => {
  window.history.replaceState({}, "", "/missing");
  renderApp();

  expect(await screen.findByRole("heading", { name: "Esta página no existe" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Volver al inicio" })).toHaveAttribute("href", "/dashboard");
  await waitFor(() => expect(document.querySelectorAll("main")).toHaveLength(1));
});

test("does not expose a public onboarding route", async () => {
  window.history.replaceState({}, "", "/onboarding");
  renderApp();

  expect(await screen.findByRole("heading", { name: "Esta página no existe" })).toBeInTheDocument();
  expect(window.location.pathname).toBe("/onboarding");
  expect(screen.queryByRole("heading", { name: /onboarding/i })).not.toBeInTheDocument();
});
