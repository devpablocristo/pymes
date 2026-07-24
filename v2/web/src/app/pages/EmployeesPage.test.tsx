import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import { I18nProvider } from "../providers/I18nProvider";
import { EmployeesPage, type EmployeeListItem } from "./EmployeesPage";

vi.mock("../shell/SectionChrome", () => ({
  SectionHeader: ({ title }: { title: string }) => <h1>{title}</h1>,
  SectionSearch: ({
    label,
    placeholder,
    value,
    onChange,
  }: {
    label: string;
    placeholder: string;
    value: string;
    onChange: (value: string) => void;
  }) => (
    <input
      aria-label={label}
      placeholder={placeholder}
      type="search"
      value={value}
      onChange={(event) => onChange(event.target.value)}
    />
  ),
}));

const employees: EmployeeListItem[] = [
  {
    id: "employee-1",
    first_name: "Ana",
    last_name: "Pérez",
    position: "Administración",
    email: "ana@example.com",
    phone: "+54 11 4444 5555",
    status: "active",
    hire_date: "2026-01-15",
  },
  {
    id: "employee-2",
    first_name: "Bruno",
    last_name: "Díaz",
    position: "Ventas",
    email: "bruno@example.com",
    phone: "+54 11 5555 6666",
    status: "inactive",
    hire_date: "2025-11-03",
  },
];

function renderPage(items: readonly EmployeeListItem[] = employees) {
  return render(
    <I18nProvider>
      <EmployeesPage employees={items} />
    </I18nProvider>,
  );
}

test("uses the Users and Tenants directory base with the employee columns", () => {
  renderPage();

  expect(screen.getByRole("table", { name: "Empleados" })).toBeInTheDocument();
  for (const heading of ["Nombre", "Puesto", "Email", "Teléfono", "Estado", "Ingreso"]) {
    expect(screen.getByRole("columnheader", { name: heading })).toBeInTheDocument();
  }
  expect(screen.getByText("Ana Pérez")).toBeInTheDocument();
  expect(screen.getByText("Administración")).toBeInTheDocument();
  expect(screen.getByText("Activo")).toBeInTheDocument();
});

test("searches across the complete employee section", async () => {
  const user = userEvent.setup();
  renderPage();

  await user.type(screen.getByRole("searchbox", { name: "Buscar empleados" }), "ventas");

  expect(screen.queryByText("Ana Pérez")).not.toBeInTheDocument();
  expect(screen.getByText("Bruno Díaz")).toBeInTheDocument();
  expect(screen.getByLabelText("Cantidad de empleados")).toHaveTextContent("1");
});

test("keeps the table structure in the empty state", () => {
  renderPage([]);

  expect(screen.getByText("No hay empleados cargados.")).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "Ingreso" })).toBeInTheDocument();
});
