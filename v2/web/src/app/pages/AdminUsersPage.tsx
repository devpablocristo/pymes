import {
  LifecycleActionToolbar,
  type LifecycleBulkAction,
  type LifecycleView,
} from "@devpablocristo/platform-lifecycle";
import { useEffect, useState, type FormEvent } from "react";
import type { components } from "../../api/schema.generated";
import { createIdempotencyKey } from "../../api/idempotency";
import { useProductApi } from "../../api/ProductApiContext";

type User = components["schemas"]["AdminUser"];
type UserList = components["schemas"]["AdminUserList"];
type TenantList = components["schemas"]["AdminTenantList"];

const stateForView: Record<LifecycleView, "active" | "archived" | "trashed"> = {
  active: "active",
  archived: "archived",
  trash: "trashed",
};

export function AdminUsersPage() {
  const api = useProductApi();
  const [items, setItems] = useState<User[]>([]);
  const [tenants, setTenants] = useState<TenantList["items"]>([]);
  const [selected, setSelected] = useState<string[]>([]);
  const [view, setView] = useState<LifecycleView>("active");
  const [createOpen, setCreateOpen] = useState(true);
  const [email, setEmail] = useState("");
  const [tenantID, setTenantID] = useState("");
  const [role, setRole] = useState<"admin" | "member">("member");
  const [editing, setEditing] = useState<User>();
  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState<string>();
  const [error, setError] = useState<string>();
  const [notice, setNotice] = useState<string>();
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    const search = new URLSearchParams({
      limit: "100",
      lifecycle_state: stateForView[view],
    });
    if (query.trim()) search.set("query", query.trim());
    Promise.all([
      api.request<UserList>(`/api/v1/admin/users?${search}`, {
        signal: controller.signal,
        skipJSONContentType: true,
      }),
      api.request<TenantList>(
        "/api/v1/admin/tenants?limit=100&status=active&lifecycle_state=active",
        { signal: controller.signal, skipJSONContentType: true },
      ),
    ])
      .then(([users, tenantList]) => {
        setItems(users.items);
        setSelected([]);
        setTenants(tenantList.items);
        setTenantID((current) => current || tenantList.items[0]?.id || "");
      })
      .catch((cause: unknown) => {
        if (!controller.signal.aborted) {
          setError(cause instanceof Error ? cause.message : "No pudimos cargar los usuarios.");
        }
      });
    return () => controller.abort();
  }, [api, query, revision, view]);

  async function command(
    operation: string,
    path: string,
    method: "POST" | "PATCH",
    body: unknown,
  ) {
    setBusy(operation);
    setError(undefined);
    setNotice(undefined);
    try {
      await api.request(path, {
        method,
        headers: { "Idempotency-Key": createIdempotencyKey(operation) },
        body: JSON.stringify(body),
      });
      setNotice("Cambio guardado.");
      setRevision((value) => value + 1);
      return true;
    } catch (cause: unknown) {
      setError(cause instanceof Error ? cause.message : "No pudimos aplicar el cambio.");
      return false;
    } finally {
      setBusy(undefined);
    }
  }

  async function invite(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const succeeded = await command("admin-user-create", "/api/v1/admin/users", "POST", {
      email: email.trim().toLowerCase(),
      tenant_id: tenantID,
      role,
    });
    if (succeeded) {
      setEmail("");
      setRole("member");
    }
  }

  async function updateUser(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!editing) return;
    const succeeded = await command(
      `admin-user-update-${editing.id}`,
      `/api/v1/admin/users/${editing.id}`,
      "PATCH",
      {
        display_name: editing.display_name.trim(),
        email: editing.email.trim().toLowerCase(),
        product_role: editing.product_role,
        version: editing.version,
      },
    );
    if (succeeded) setEditing(undefined);
  }

  async function lifecycleAction(action: LifecycleBulkAction) {
    const endpoint =
      action === "restore" && view === "archived" ? "unarchive" : action;
    const method = endpoint === "purge" ? "DELETE" : "POST";
    const targets = [...selected];
    setBusy(`admin-user-${endpoint}`);
    setError(undefined);
    setNotice(undefined);
    try {
      await Promise.all(
        targets.map((id) =>
          api.request(`/api/v1/admin/users/${id}/${endpoint}`, {
            method,
            headers: {
              "Idempotency-Key": createIdempotencyKey(`admin-user-${endpoint}-${id}`),
            },
            body: JSON.stringify({}),
          }),
        ),
      );
      setSelected([]);
      setNotice(targets.length === 1 ? "Cambio guardado." : `${targets.length} usuarios actualizados.`);
      setRevision((value) => value + 1);
    } catch (cause: unknown) {
      setError(cause instanceof Error ? cause.message : "No pudimos aplicar el cambio.");
    } finally {
      setBusy(undefined);
    }
  }

  const selectedUser = selected.length === 1
    ? items.find((user) => user.id === selected[0])
    : undefined;

  return (
    <div className="settings-page">
      <header className="page-topbar">
        <div>
          <h1>Usuarios</h1>
          <small>Administración global</small>
        </div>
      </header>
      <div className="settings-canvas admin-canvas">
        <div className="settings-heading">
          <div>
            <h2>Usuarios de Pymes</h2>
            <p>Owners globales y lifecycle canónico de Platform.</p>
          </div>
          <span className="settings-count">{items.length}</span>
        </div>

        <div className="admin-lifecycle-tabs" role="tablist" aria-label="Estado de usuarios">
          {(["active", "archived", "trash"] as LifecycleView[]).map((candidate) => (
            <button
              aria-selected={view === candidate}
              className={view === candidate ? "is-active" : ""}
              key={candidate}
              onClick={() => setView(candidate)}
              role="tab"
              type="button"
            >
              {candidate === "active" ? "Activos" : candidate === "archived" ? "Archivados" : "Papelera"}
            </button>
          ))}
        </div>

        {error ? <div className="inline-state inline-state--error" role="alert">{error}</div> : null}
        {notice ? <div className="inline-state inline-state--success" role="status">{notice}</div> : null}

        <LifecycleActionToolbar
          busy={Boolean(busy)}
          createOpen={createOpen}
          editOpen={Boolean(editing)}
          onBulkAction={(action) => void lifecycleAction(action)}
          onClear={() => setSelected([])}
          onCreate={() => {
            setView("active");
            setCreateOpen((value) => !value);
          }}
          onEdit={() => selectedUser && setEditing(selectedUser)}
          selectedCount={selected.length}
          view={view}
          labels={{
            newButton: "Invitar usuario",
            editButton: "Editar",
            clearButton: "Limpiar selección",
            archiveButton: "Archivar",
            trashButton: "Papelera",
            restoreButton: "Restaurar",
            deleteButton: "Eliminar definitivamente",
            selectedSuffix: "seleccionados",
          }}
          classNames={{
            root: "admin-lifecycle-toolbar",
            buttons: "admin-lifecycle-toolbar__buttons",
            group: "admin-lifecycle-toolbar__group",
            dangerButton: "settings-row__danger",
            selectedCount: "admin-lifecycle-toolbar__count",
          }}
        />

        {createOpen && view === "active" ? (
          <form className="admin-create-form" onSubmit={(event) => void invite(event)}>
            <label>
              <span>Email</span>
              <input required type="email" value={email} onChange={(event) => setEmail(event.target.value)} />
            </label>
            <label>
              <span>Tenant</span>
              <select required value={tenantID} onChange={(event) => setTenantID(event.target.value)}>
                {tenants.map((tenant) => <option value={tenant.id} key={tenant.id}>{tenant.name}</option>)}
              </select>
            </label>
            <label>
              <span>Rol local</span>
              <select value={role} onChange={(event) => setRole(event.target.value as "admin" | "member")}>
                <option value="member">Miembro</option>
                <option value="admin">Administrador</option>
              </select>
            </label>
            <button disabled={Boolean(busy) || !tenantID} type="submit">
              {busy === "admin-user-create" ? "Invitando…" : "Invitar usuario"}
            </button>
          </form>
        ) : null}

        <div className="admin-toolbar">
          <input type="search" placeholder="Buscar por nombre o email" value={query} onChange={(event) => setQuery(event.target.value)} />
        </div>

        <div className="settings-list">
          {items.map((user) => (
            <article className="settings-row admin-row admin-user-row" key={user.id}>
              <input
                aria-label={`Seleccionar ${user.display_name}`}
                checked={selected.includes(user.id)}
                onChange={(event) =>
                  setSelected((current) =>
                    event.target.checked
                      ? [...current, user.id]
                      : current.filter((id) => id !== user.id),
                  )
                }
                type="checkbox"
              />
              <span className="settings-row__avatar" aria-hidden="true">
                {user.display_name.slice(0, 1).toUpperCase()}
              </span>
              <div className="settings-row__body">
                <div className="settings-row__title">
                  <strong>{user.display_name}</strong>
                  <span className={`status-pill status-pill--${user.lifecycle_state}`}>
                    {user.lifecycle_state}
                  </span>
                </div>
                <span>{user.email}</span>
                <small>
                  {user.product_role === "owner" ? "Owner global" : "Usuario"} · Estado operativo: {user.status} ·{" "}
                  {user.memberships.map((membership) => membership.tenant_name).join(", ") || "Sin tenant"}
                </small>
              </div>
            </article>
          ))}
        </div>
      </div>

      {editing ? (
        <div className="admin-dialog-backdrop" role="presentation">
          <form className="admin-dialog" onSubmit={(event) => void updateUser(event)}>
            <h2>Editar usuario</h2>
            <label>
              <span>Nombre</span>
              <input required value={editing.display_name} onChange={(event) => setEditing({ ...editing, display_name: event.target.value })} />
            </label>
            <label>
              <span>Email</span>
              <input required type="email" value={editing.email} onChange={(event) => setEditing({ ...editing, email: event.target.value })} />
            </label>
            <label>
              <span>Rol global</span>
              <select
                value={editing.product_role}
                onChange={(event) => setEditing({ ...editing, product_role: event.target.value as "owner" | "user" })}
              >
                <option value="user">Usuario</option>
                <option value="owner">Owner global</option>
              </select>
            </label>
            <div className="admin-dialog__actions">
              <button type="button" onClick={() => setEditing(undefined)}>Cancelar</button>
              <button type="submit" disabled={Boolean(busy)}>Guardar</button>
            </div>
          </form>
        </div>
      ) : null}
    </div>
  );
}
