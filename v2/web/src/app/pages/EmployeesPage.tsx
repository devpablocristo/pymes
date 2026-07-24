import { useMemo, useState } from "react";
import { useI18n } from "../providers/I18nProvider";
import { SectionHeader, SectionSearch } from "../shell/SectionChrome";

export type EmployeeListItem = {
  id: string;
  first_name: string;
  last_name?: string | null;
  position?: string | null;
  email?: string | null;
  phone?: string | null;
  status: string;
  hire_date?: string | null;
};

type EmployeesPageProps = {
  employees?: readonly EmployeeListItem[];
};

const statusLabels: Record<string, string> = {
  active: "Activo",
  inactive: "Inactivo",
  terminated: "Baja",
};

export function EmployeesPage({ employees = [] }: EmployeesPageProps) {
  const { language, t } = useI18n();
  const [search, setSearch] = useState("");
  const visibleEmployees = useMemo(() => {
    const locale = language === "es" ? "es" : "en";
    const query = search.trim().toLocaleLowerCase(locale);
    if (!query) return employees;
    return employees.filter((employee) =>
      [
        employee.first_name,
        employee.last_name,
        employee.position,
        employee.email,
        employee.phone,
        statusLabels[employee.status] ?? employee.status,
        employee.hire_date,
      ].some((value) => String(value ?? "").toLocaleLowerCase(locale).includes(query)),
    );
  }, [employees, language, search]);

  return (
    <div className="directory-page employees-page">
      <SectionHeader title={t("nav.employees")} subtitle={t("nav.teamSection")} />
      <div className="directory-page__content">
        <section className="directory-section">
          <div className="directory-section__heading">
            <div className="directory-section__title">
              <h2>{t("nav.employees")}</h2>
              <span className="settings-count" aria-label="Cantidad de empleados">
                {visibleEmployees.length}
              </span>
            </div>
            <div className="directory-section__actions">
              <SectionSearch
                label="Buscar empleados"
                placeholder="Buscar empleados…"
                value={search}
                onChange={setSearch}
              />
            </div>
          </div>

          <div className="directory-table-wrap">
            <table className="directory-table" aria-label={t("nav.employees")}>
              <thead>
                <tr>
                  <th>Nombre</th>
                  <th>Puesto</th>
                  <th>Email</th>
                  <th>Teléfono</th>
                  <th>Estado</th>
                  <th>Ingreso</th>
                </tr>
              </thead>
              <tbody>
                {visibleEmployees.map((employee) => {
                  const fullName = [employee.first_name, employee.last_name]
                    .filter(Boolean)
                    .join(" ");
                  return (
                    <tr key={employee.id}>
                      <td className="directory-table__primary">{fullName || "—"}</td>
                      <td>{employee.position || "—"}</td>
                      <td>{employee.email || "—"}</td>
                      <td>{employee.phone || "—"}</td>
                      <td>
                        <span className={`status-pill status-pill--${employee.status}`}>
                          {statusLabels[employee.status] ?? employee.status}
                        </span>
                      </td>
                      <td>{formatHireDate(employee.hire_date, language)}</td>
                    </tr>
                  );
                })}
                {visibleEmployees.length === 0 ? (
                  <tr>
                    <td className="directory-empty" colSpan={6}>
                      {employees.length === 0
                        ? "No hay empleados cargados."
                        : "No hay empleados que coincidan con la búsqueda."}
                    </td>
                  </tr>
                ) : null}
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </div>
  );
}

function formatHireDate(value: string | null | undefined, language: string) {
  if (!value) return "—";
  const date = new Date(`${value.slice(0, 10)}T00:00:00`);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(language === "es" ? "es-AR" : "en-US", {
    dateStyle: "medium",
  }).format(date);
}
