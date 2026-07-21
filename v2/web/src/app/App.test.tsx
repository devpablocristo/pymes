import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";
import { App } from "./App";
import { AppProviders } from "./providers/AppProviders";

function renderApp() {
  return render(
    <AppProviders>
      <App />
    </AppProviders>,
  );
}

test("renders the technical shell", () => {
  renderApp();

  expect(screen.getByRole("heading", { name: "Una base limpia para construir." })).toBeInTheDocument();
  expect(screen.getByRole("navigation", { name: "Navegación principal" })).toBeInTheDocument();
  expect(screen.getByText("PostgreSQL")).toBeInTheDocument();
});

test("switches theme and locale through providers", async () => {
  const user = userEvent.setup();
  renderApp();

  await user.click(screen.getByRole("button", { name: "Usar tema oscuro" }));
  expect(document.documentElement).toHaveAttribute("data-theme", "dark");

  await user.selectOptions(screen.getByRole("combobox", { name: "Idioma" }), "en");
  expect(screen.getByRole("heading", { name: "A clean foundation to build on." })).toBeInTheDocument();
  expect(document.documentElement).toHaveAttribute("lang", "en");
});
