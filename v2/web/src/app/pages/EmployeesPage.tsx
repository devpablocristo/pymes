import { useEffect, useMemo, useState } from "react";
import {
  EntityLifecycleBulkToolbar,
  EntityLifecycleTabs,
  type EntityLifecycleAction,
  type EntityLifecycleState,
} from "../components/EntityLifecycle";
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
  lifecycle_state?: "active" | "archived" | "trashed";
};

type EmployeesPageProps = {
  employees?: readonly EmployeeListItem[];
  onLifecycle?: (
    employee: EmployeeListItem,
    action: EntityLifecycleAction,
  ) => Promise<void> | void;
};

const statusLabels: Record<string, string> = {
  active: "Activo",
  inactive: "Inactivo",
  terminated: "Baja",
};

const emptyEmployees: readonly EmployeeListItem[] = [];

export function EmployeesPage({
  employees = emptyEmployees,
  onLifecycle,
}: EmployeesPageProps) {
  const { language, t } = useI18n();
  const [search, setSearch] = useState("");
  const [state, setState] = useState<EntityLifecycleState>("active");
  const [items, setItems] = useState<EmployeeListItem[]>([...employees]);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [busy, setBusy] = useState<string>();
  const [error, setError] = useState<string>();

  useEffect(() => setItems([...employees]), [employees]);

  const visibleEmployees = useMemo(() => {
    const locale = language === "es" ? "es" : "en";
    const query = search.trim().toLocaleLowerCase(locale);
    const lifecycleState = state === "trash" ? "trashed" : state;
    return items.filter(
      (employee) =>
        (employee.lifecycle_state ?? "active") === lifecycleState &&
        (!query ||
          [
            employee.first_name,
            employee.last_name,
            employee.position,
            employee.email,
            employee.phone,
            statusLabels[employee.status] ?? employee.status,
            employee.hire_date,
          ].some((value) =>
            String(value ?? "").toLocaleLowerCase(locale).includes(query),
          )),
    );
  }, [items, language, search, state]);

  async function applyLifecycle(action: EntityLifecycleAction) {
    if (selectedIds.length === 0) return;
    if (
      action === "purge" &&
      !window.confirm("Esta acción elimina el empleado definitivamente. ¿Continuar?")
    ) {
      return;
    }
    setBusy(`bulk-${action}`);
    setError(undefined);
    try {
      for (const employeeID of selectedIds) {
        const employee = items.find((candidate) => candidate.id === employeeID);
        if (employee) await onLifecycle?.(employee, action);
      }
      if (action === "purge") {
        setItems((current) =>
          current.filter((item) => !selectedIds.includes(item.id)),
        );
        setSelectedIds([]);
        return;
      }
      const lifecycleState =
        action === "archive"
          ? "archived"
          : action === "trash"
            ? "trashed"
            : "active";
      setItems((current) =>
        current.map((item) =>
          selectedIds.includes(item.id)
            ? { ...item, lifecycle_state: lifecycleState }
            : item,
        ),
      );
      setSelectedIds([]);
    } catch (cause: unknown) {
      setError(
        cause instanceof Error
          ? cause.message
          : "No pudimos cambiar el estado del empleado.",
      );
    } finally {
      setBusy(undefined);
    }
  }

  return (
    <div className="directory-page employees-page">
      <SectionHeader title={t("nav.employees")} subtitle={t("nav.teamSection")} />
      <div className="directory-page__content">
        <section className="directory-section">
          <div className="directory-section__heading">
            <div className="directory-section__controls">
              <EntityLifecycleBulkToolbar
                busy={Boolean(busy)}
                onAction={(action) => void applyLifecycle(action)}
                onClear={() => setSelectedIds([])}
                selectedCount={selectedIds.length}
                state={state}
              />
              <div className="directory-section__filter-group">
                <SectionSearch
                  label="Buscar empleados"
                  placeholder="Buscar empleados…"
                  value={search}
                  onChange={setSearch}
                />
                <EntityLifecycleTabs
                  label="Estado de empleados"
                  state={state}
                  onChange={(nextState) => {
                    setState(nextState);
                    setSelectedIds([]);
                  }}
                />
              </div>
            </div>
          </div>

          {error ? (
            <div className="inline-state inline-state--error" role="alert">
              {error}
            </div>
          ) : null}

          <div className="directory-table-wrap">
            <table className="directory-table" aria-label={t("nav.employees")}>
              <thead>
                <tr>
                  <th className="entity-select-cell" />
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
                      <td className="entity-select-cell">
                        <input
                          aria-label={`Seleccionar ${fullName || employee.id}`}
                          checked={selectedIds.includes(employee.id)}
                          onChange={(event) => {
                            const checked = event.currentTarget.checked;
                            setSelectedIds((current) =>
                              checked
                                ? Array.from(new Set([...current, employee.id]))
                                : current.filter((id) => id !== employee.id),
                            );
                          }}
                          type="checkbox"
                        />
                      </td>
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
                    <td className="directory-empty" colSpan={7}>
                      <strong>
                        {items.length === 0
                          ? "Todavía no hay empleados"
                          : "No encontramos empleados"}
                      </strong>
                      <span>
                        {items.length === 0
                          ? "Cuando agregues empleados, aparecerán en esta lista."
                          : "Probá con otra búsqueda o cambiá el estado seleccionado."}
                      </span>
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
