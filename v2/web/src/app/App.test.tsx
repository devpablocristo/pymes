import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, test } from "vitest";
import { App } from "./App";
import { AppProviders } from "./providers/AppProviders";

function renderApp() {
  return render(
    <AppProviders>
      <App />
    </AppProviders>,
  );
}

beforeEach(() => {
  window.localStorage.clear();
  window.history.replaceState({}, "", "/");
});

test("redirects home into the product console with one main landmark", async () => {
  renderApp();

  expect(await screen.findByRole("heading", { name: "Inicio" })).toBeInTheDocument();
  expect(window.location.pathname).toBe("/dashboard");
  expect(document.querySelectorAll("main")).toHaveLength(1);
  expect(screen.getByRole("link", { name: "Inicio" })).toHaveAttribute("aria-current", "page");
  expect(screen.queryByText("Clientes")).not.toBeInTheDocument();
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
