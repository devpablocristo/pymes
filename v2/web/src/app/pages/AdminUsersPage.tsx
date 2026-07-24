import { useEffect, useMemo, useState, type FormEvent } from "react";
import type { components } from "../../api/schema.generated";
import { createIdempotencyKey } from "../../api/idempotency";
import { useProductApi } from "../../api/ProductApiContext";
import { useProductAuth } from "../../auth/AuthContext";
import { SectionHeader, SectionSearch } from "../shell/SectionChrome";
import { AdminDirectoryTabs } from "./AdminDirectoryTabs";

type User = components["schemas"]["AdminUser"];
type UserList = components["schemas"]["AdminUserList"];
type TenantList = components["schemas"]["AdminTenantList"];
type View = "active" | "archived" | "trashed";

const viewLabels: Record<View, string> = {
  active: "Activos",
  archived: "Archivados",
  trashed: "Papelera",
};

export function AdminUsersPage() {
  const api = useProductApi();
  const auth = useProductAuth();
  const [items, setItems] = useState<User[]>([]);
  const [tenants, setTenants] = useState<TenantList["items"]>([]);
  const [view, setView] = useState<View>("active");
  const [createOpen, setCreateOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [tenantID, setTenantID] = useState("");
  const [role, setRole] = useState<"admin" | "member">("member");
  const [editing, setEditing] = useState<User>();
  const [busy, setBusy] = useState<string>();
  const [error, setError] = useState<string>();
  const [notice, setNotice] = useState<string>();
  const [revision, setRevision] = useState(0);
  const [search, setSearch] = useState("");
  const visibleItems = useMemo(() => {
    const query = search.trim().toLocaleLowerCase("es");
    if (!query) return items;
    return items.filter((user) =>
      [
        user.email,
        user.display_name,
        user.product_role,
        ...user.memberships.flatMap((membership) => [
          membership.role,
          membership.tenant_name,
        ]),
      ].some((value) => value.toLocaleLowerCase("es").includes(query)),
    );
  }, [items, search]);

  useEffect(() => {
    const controller = new AbortController();
    Promise.all([
      api.request<UserList>(
        `/api/v1/admin/users?limit=100&lifecycle_state=${view}`,
        { signal: controller.signal, skipJSONContentType: true },
      ),
      api.request<TenantList>(
        "/api/v1/admin/tenants?limit=100&status=active&lifecycle_state=active",
        { signal: controller.signal, skipJSONContentType: true },
      ),
    ])
      .then(([users, tenantList]) => {
        setItems(users.items);
        setTenants(tenantList.items);
        setTenantID((current) => current || tenantList.items[0]?.id || "");
      })
      .catch((cause: unknown) => {
        if (!controller.signal.aborted) {
          setError(cause instanceof Error ? cause.message : "No pudimos cargar los usuarios.");
        }
      });
    return () => controller.abort();
  }, [api, revision, view]);

  async function request(
    operation: string,
    path: string,
    method: "POST" | "PATCH" | "DELETE",
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
    const succeeded = await request("admin-user-create", "/api/v1/admin/users", "POST", {
      email: email.trim().toLowerCase(),
      tenant_id: tenantID,
      role,
    });
    if (succeeded) {
      setEmail("");
      setRole("member");
      setCreateOpen(false);
    }
  }

  async function updateUser(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!editing) return;
    const succeeded = await request(
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

  async function lifecycle(user: User, action: "archive" | "trash" | "unarchive" | "restore" | "purge") {
    await request(
      `admin-user-${action}-${user.id}`,
      `/api/v1/admin/users/${user.id}/${action}`,
      action === "purge" ? "DELETE" : "POST",
      {},
    );
  }

  return (
    <div className="directory-page">
      <SectionHeader title="Usuarios y Tenants" />
      <div className="directory-page__content">
        <AdminDirectoryTabs />

        <section className="directory-section">
          <div className="directory-section__heading">
            <div className="directory-section__title">
              <h2>Usuarios</h2>
              <div className="lifecycle-tabs" role="tablist" aria-label="Estado de usuarios">
                {(Object.keys(viewLabels) as View[]).map((candidate) => (
                  <button
                    aria-selected={view === candidate}
                    className={view === candidate ? "is-active" : ""}
                    key={candidate}
                    onClick={() => setView(candidate)}
                    role="tab"
                    type="button"
                  >
                    {viewLabels[candidate]}
                  </button>
                ))}
              </div>
            </div>
            <div className="directory-section__actions">
              <SectionSearch
                label="Buscar usuarios"
                placeholder="Buscar usuarios…"
                value={search}
                onChange={setSearch}
              />
              <button className="directory-create-button" onClick={() => setCreateOpen(true)} type="button">
                <span aria-hidden="true">＋</span> Crear
              </button>
            </div>
          </div>

          {error ? <div className="inline-state inline-state--error" role="alert">{error}</div> : null}
          {notice ? <div className="inline-state inline-state--success" role="status">{notice}</div> : null}

          <div className="directory-table-wrap">
            <table className="directory-table">
              <thead>
                <tr>
                  <th>Email</th>
                  <th>Usuario</th>
                  <th>Rol</th>
                  <th>Acciones</th>
                </tr>
              </thead>
              <tbody>
                {visibleItems.map((user) => (
                  <tr key={user.id}>
                    <td>
                      {user.email}
                      {auth.user?.email?.toLowerCase() === user.email.toLowerCase() ? (
                        <small className="directory-self"> (vos)</small>
                      ) : null}
                    </td>
                    <td>{user.display_name}</td>
                    <td>{user.product_role === "owner" ? "owner" : user.memberships[0]?.role || "usuario"}</td>
                    <td>
                      <div className="directory-row-actions">
                        {view === "active" ? (
                          <>
                            <button onClick={() => setEditing(user)} type="button">Editar</button>
                            <button onClick={() => void lifecycle(user, "archive")} type="button">Archivar</button>
                            <button className="is-danger" onClick={() => void lifecycle(user, "trash")} type="button">Papelera</button>
                          </>
                        ) : null}
                        {view === "archived" ? (
                          <button onClick={() => void lifecycle(user, "unarchive")} type="button">Restaurar</button>
                        ) : null}
                        {view === "trashed" ? (
                          <>
                            <button onClick={() => void lifecycle(user, "restore")} type="button">Restaurar</button>
                            <button className="is-danger" onClick={() => void lifecycle(user, "purge")} type="button">Eliminar</button>
                          </>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                ))}
                {visibleItems.length === 0 ? (
                  <tr>
                    <td className="directory-empty" colSpan={4}>
                      {items.length === 0 ? "No hay usuarios en esta vista." : "No hay usuarios que coincidan con la búsqueda."}
                    </td>
                  </tr>
                ) : null}
              </tbody>
            </table>
          </div>
        </section>
      </div>

      {createOpen ? (
        <div className="admin-dialog-backdrop" role="presentation">
          <form className="admin-dialog" onSubmit={(event) => void invite(event)}>
            <h2>Crear usuario</h2>
            <label><span>Email</span><input required type="email" value={email} onChange={(event) => setEmail(event.target.value)} /></label>
            <label>
              <span>Tenant</span>
              <select required value={tenantID} onChange={(event) => setTenantID(event.target.value)}>
                {tenants.map((tenant) => <option value={tenant.id} key={tenant.id}>{tenant.name}</option>)}
              </select>
            </label>
            <label>
              <span>Rol</span>
              <select value={role} onChange={(event) => setRole(event.target.value as "admin" | "member")}>
                <option value="member">Miembro</option>
                <option value="admin">Administrador</option>
              </select>
            </label>
            <div className="admin-dialog__actions">
              <button type="button" onClick={() => setCreateOpen(false)}>Cancelar</button>
              <button type="submit" disabled={Boolean(busy) || !tenantID}>Crear usuario</button>
            </div>
          </form>
        </div>
      ) : null}

      {editing ? (
        <div className="admin-dialog-backdrop" role="presentation">
          <form className="admin-dialog" onSubmit={(event) => void updateUser(event)}>
            <h2>Editar usuario</h2>
            <label><span>Nombre</span><input required value={editing.display_name} onChange={(event) => setEditing({ ...editing, display_name: event.target.value })} /></label>
            <label><span>Email</span><input required type="email" value={editing.email} onChange={(event) => setEditing({ ...editing, email: event.target.value })} /></label>
            <label>
              <span>Rol global</span>
              <select value={editing.product_role} onChange={(event) => setEditing({ ...editing, product_role: event.target.value as "owner" | "user" })}>
                <option value="user">Usuario</option>
                <option value="owner">Owner</option>
              </select>
            </label>
            <div className="admin-dialog__actions">
              <button type="button" onClick={() => setEditing(undefined)}>Cancelar</button>
              <button type="submit" disabled={Boolean(busy)}>Guardar cambios</button>
            </div>
          </form>
        </div>
      ) : null}
    </div>
  );
}
