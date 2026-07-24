import {
  matchesAccessSearch,
  normalizeAccessSearch,
  type AccessTab,
  type ResourceState,
} from "@devpablocristo/platform-access-management";
import "@devpablocristo/platform-access-management/styles.css";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import type { components } from "../../api/schema.generated";
import { createIdempotencyKey } from "../../api/idempotency";
import { useProductApi } from "../../api/ProductApiContext";
import { useProductAuth } from "../../auth/AuthContext";
import { SectionHeader } from "../shell/SectionChrome";

type AdminTenant = components["schemas"]["AdminTenant"];
type AdminTenantList = components["schemas"]["AdminTenantList"];
type AdminUser = components["schemas"]["AdminUser"];
type AdminUserList = components["schemas"]["AdminUserList"];
type Invitation = components["schemas"]["Invitation"];
type InvitationList = components["schemas"]["InvitationList"];

type LifecycleAction = "archive" | "unarchive" | "trash" | "restore" | "purge";

const tabLabels: Record<AccessTab, string> = {
  users: "Usuarios",
  tenants: "Tenants",
  invitations: "Invitaciones",
};

const resourceStateLabels: Record<ResourceState, string> = {
  active: "Activos",
  archived: "Archivados",
  trash: "Papelera",
};

function apiLifecycleState(state: ResourceState) {
  return state === "trash" ? "trashed" : state;
}

export function AccessManagementPage() {
  const api = useProductApi();
  const auth = useProductAuth();
  const [activeTab, setActiveTab] = useState<AccessTab>("users");
  const [resourceStates, setResourceStates] = useState<
    Record<"users" | "tenants", ResourceState>
  >({
    users: "active",
    tenants: "active",
  });
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [tenants, setTenants] = useState<AdminTenant[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>();
  const [notice, setNotice] = useState<string>();
  const [busy, setBusy] = useState<string>();
  const [revision, setRevision] = useState(0);
  const [editingTenant, setEditingTenant] = useState<AdminTenant>();
  const [invitationOpen, setInvitationOpen] = useState(false);
  const [invitationEmail, setInvitationEmail] = useState("");
  const [invitationRole, setInvitationRole] = useState<"admin" | "member">("member");

  const activeOrganization = auth.organizations.find(
    (organization) => organization.id === auth.activeOrganizationId,
  );
  const selectedResourceState =
    activeTab === "invitations" ? undefined : resourceStates[activeTab];

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(undefined);

    const request =
      activeTab === "users"
        ? api
            .request<AdminUserList>(
              `/api/v1/admin/users?limit=100&lifecycle_state=${apiLifecycleState(resourceStates.users)}`,
              { signal: controller.signal, skipJSONContentType: true },
            )
            .then((response) => setUsers(response.items))
        : activeTab === "tenants"
          ? api
              .request<AdminTenantList>(
                `/api/v1/admin/tenants?limit=100&lifecycle_state=${apiLifecycleState(resourceStates.tenants)}`,
                { signal: controller.signal, skipJSONContentType: true },
              )
              .then((response) => setTenants(response.items))
          : activeOrganization
            ? api
                .request<InvitationList>("/api/v1/team/invitations?limit=100", {
                  signal: controller.signal,
                  skipJSONContentType: true,
                })
                .then((response) => setInvitations(response.items))
            : Promise.resolve(setInvitations([]));

    request
      .catch((cause: unknown) => {
        if (!controller.signal.aborted) {
          setError(
            cause instanceof Error
              ? cause.message
              : "No pudimos cargar esta sección.",
          );
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });

    return () => controller.abort();
  }, [
    activeOrganization,
    activeTab,
    api,
    resourceStates.tenants,
    resourceStates.users,
    revision,
  ]);

  const normalizedQuery = normalizeAccessSearch(query);
  const visibleUsers = useMemo(
    () =>
      users.filter((user) =>
        matchesAccessSearch(normalizedQuery, [
          user.email,
          user.display_name,
          user.product_role,
          ...user.memberships.flatMap((membership) => [
            membership.tenant_name,
            membership.role,
          ]),
        ]),
      ),
    [normalizedQuery, users],
  );
  const visibleTenants = useMemo(
    () =>
      tenants.filter((tenant) =>
        matchesAccessSearch(normalizedQuery, [
          tenant.name,
          tenant.slug,
          tenant.status,
        ]),
      ),
    [normalizedQuery, tenants],
  );
  const visibleInvitations = useMemo(
    () =>
      invitations.filter((invitation) =>
        matchesAccessSearch(normalizedQuery, [
          invitation.email,
          invitation.role,
          invitation.status,
          invitation.sync_status,
        ]),
      ),
    [invitations, normalizedQuery],
  );

  async function command(
    operation: string,
    path: string,
    method: "POST" | "PATCH" | "DELETE",
    body: unknown = {},
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
      setRevision((current) => current + 1);
      return true;
    } catch (cause: unknown) {
      setError(
        cause instanceof Error ? cause.message : "No pudimos aplicar el cambio.",
      );
      return false;
    } finally {
      setBusy(undefined);
    }
  }

  async function lifecycle(
    resource: "users" | "tenants",
    resourceId: string,
    action: LifecycleAction,
  ) {
    if (
      action === "purge" &&
      !window.confirm("Esta acción elimina el registro definitivamente. ¿Continuar?")
    ) {
      return;
    }
    await command(
      `admin-${resource}-${action}-${resourceId}`,
      `/api/v1/admin/${resource}/${encodeURIComponent(resourceId)}/${action}`,
      action === "purge" ? "DELETE" : "POST",
    );
  }

  async function updateTenant(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!editingTenant) return;
    const succeeded = await command(
      `admin-tenant-update-${editingTenant.id}`,
      `/api/v1/admin/tenants/${encodeURIComponent(editingTenant.id)}`,
      "PATCH",
      {
        name: editingTenant.name.trim(),
        slug: editingTenant.slug.trim().toLowerCase(),
      },
    );
    if (succeeded) setEditingTenant(undefined);
  }

  async function createInvitation(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const succeeded = await command(
      "team-invitation-create",
      "/api/v1/team/invitations",
      "POST",
      {
        email: invitationEmail.trim().toLowerCase(),
        role: invitationRole,
      },
    );
    if (succeeded) {
      setInvitationEmail("");
      setInvitationRole("member");
      setInvitationOpen(false);
    }
  }

  function selectTab(tab: AccessTab) {
    setActiveTab(tab);
    setQuery("");
    setError(undefined);
    setNotice(undefined);
  }

  return (
    <div className="directory-page access-management-page">
      <SectionHeader title="Usuarios y Tenants" />
      <div className="directory-page__content access-management-page__content">
        <section className="platform-access">
          <header className="platform-access__bar">
            <nav className="platform-access__tabs" aria-label="Gestión de acceso">
              {(Object.keys(tabLabels) as AccessTab[]).map((tab) => (
                <button
                  aria-selected={activeTab === tab}
                  className={`platform-access__tab ${activeTab === tab ? "is-active" : ""}`}
                  key={tab}
                  onClick={() => selectTab(tab)}
                  role="tab"
                  type="button"
                >
                  {tabLabels[tab]}
                </button>
              ))}
            </nav>
            <label className="platform-access__search">
              <span aria-hidden="true">⌕</span>
              <input
                aria-label={`Buscar ${tabLabels[activeTab].toLowerCase()}`}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Buscar…"
                type="search"
                value={query}
              />
            </label>
          </header>

          <div className="platform-access__panel">
            <div className="platform-access__heading access-management-page__heading">
              <div>
                <h2>{tabLabels[activeTab]}</h2>
                {selectedResourceState ? (
                  <div
                    className="lifecycle-tabs"
                    role="tablist"
                    aria-label={`Estado de ${tabLabels[activeTab].toLowerCase()}`}
                  >
                    {(Object.keys(resourceStateLabels) as ResourceState[]).map(
                      (state) => (
                        <button
                          aria-selected={selectedResourceState === state}
                          className={
                            selectedResourceState === state ? "is-active" : ""
                          }
                          key={state}
                          onClick={() =>
                            setResourceStates((current) => ({
                              ...current,
                              [activeTab]: state,
                            }))
                          }
                          role="tab"
                          type="button"
                        >
                          {resourceStateLabels[state]}
                        </button>
                      ),
                    )}
                  </div>
                ) : null}
              </div>

              {activeTab === "invitations" && activeOrganization ? (
                <button
                  className="platform-access__primary"
                  onClick={() => setInvitationOpen(true)}
                  type="button"
                >
                  + Nueva invitación
                </button>
              ) : null}
            </div>

            {error ? (
              <div className="platform-access__feedback" role="alert">
                {error}
              </div>
            ) : null}
            {notice ? (
              <div className="inline-state inline-state--success" role="status">
                {notice}
              </div>
            ) : null}

            {loading ? (
              <div className="platform-access__loading" aria-live="polite">
                Cargando…
              </div>
            ) : (
              <>
                {activeTab === "users" ? (
                  <UsersTable
                    busy={busy}
                    currentEmail={auth.user?.email}
                    items={visibleUsers}
                    state={resourceStates.users}
                    onLifecycle={(id, action) =>
                      void lifecycle("users", id, action)
                    }
                  />
                ) : null}
                {activeTab === "tenants" ? (
                  <TenantsTable
                    busy={busy}
                    items={visibleTenants}
                    state={resourceStates.tenants}
                    onEdit={setEditingTenant}
                    onLifecycle={(id, action) =>
                      void lifecycle("tenants", id, action)
                    }
                  />
                ) : null}
                {activeTab === "invitations" ? (
                  <InvitationsTable
                    busy={busy}
                    items={visibleInvitations}
                    onResend={(id) =>
                      void command(
                        `team-invitation-resend-${id}`,
                        `/api/v1/team/invitations/${encodeURIComponent(id)}/resend`,
                        "POST",
                      )
                    }
                    onRevoke={(id) =>
                      void command(
                        `team-invitation-revoke-${id}`,
                        `/api/v1/team/invitations/${encodeURIComponent(id)}/revoke`,
                        "POST",
                      )
                    }
                  />
                ) : null}
              </>
            )}
          </div>
        </section>
      </div>

      {editingTenant ? (
        <div
          className="platform-access__backdrop"
          onMouseDown={() => setEditingTenant(undefined)}
          role="presentation"
        >
          <div
            aria-label="Editar tenant"
            aria-modal="true"
            className="platform-access__dialog"
            onMouseDown={(event) => event.stopPropagation()}
            role="dialog"
          >
            <h2>Editar tenant</h2>
            <form onSubmit={(event) => void updateTenant(event)}>
              <label>
                Nombre del tenant
                <input
                  required
                  value={editingTenant.name}
                  onChange={(event) =>
                    setEditingTenant({
                      ...editingTenant,
                      name: event.target.value,
                    })
                  }
                />
              </label>
              <label>
                Slug
                <input
                  pattern="[a-z0-9]+(?:-[a-z0-9]+)*"
                  required
                  value={editingTenant.slug}
                  onChange={(event) =>
                    setEditingTenant({
                      ...editingTenant,
                      slug: event.target.value.toLowerCase(),
                    })
                  }
                />
              </label>
              <div className="platform-access__dialog-actions">
                <button type="button" onClick={() => setEditingTenant(undefined)}>
                  Cancelar
                </button>
                <button
                  className="platform-access__primary"
                  disabled={Boolean(busy)}
                  type="submit"
                >
                  Guardar
                </button>
              </div>
            </form>
          </div>
        </div>
      ) : null}

      {invitationOpen ? (
        <div
          className="platform-access__backdrop"
          onMouseDown={() => setInvitationOpen(false)}
          role="presentation"
        >
          <div
            aria-label="Nueva invitación"
            aria-modal="true"
            className="platform-access__dialog"
            onMouseDown={(event) => event.stopPropagation()}
            role="dialog"
          >
            <h2>Nueva invitación</h2>
            <form onSubmit={(event) => void createInvitation(event)}>
              <label>
                Email
                <input
                  required
                  type="email"
                  value={invitationEmail}
                  onChange={(event) => setInvitationEmail(event.target.value)}
                />
              </label>
              <label>
                Rol
                <select
                  value={invitationRole}
                  onChange={(event) =>
                    setInvitationRole(event.target.value as "admin" | "member")
                  }
                >
                  <option value="member">Miembro</option>
                  <option value="admin">Administrador</option>
                </select>
              </label>
              <div className="platform-access__dialog-actions">
                <button type="button" onClick={() => setInvitationOpen(false)}>
                  Cancelar
                </button>
                <button
                  className="platform-access__primary"
                  disabled={Boolean(busy)}
                  type="submit"
                >
                  Enviar invitación
                </button>
              </div>
            </form>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function UsersTable({
  busy,
  currentEmail,
  items,
  onLifecycle,
  state,
}: {
  busy?: string;
  currentEmail?: string;
  items: AdminUser[];
  onLifecycle: (id: string, action: LifecycleAction) => void;
  state: ResourceState;
}) {
  return (
    <div className="directory-table-wrap">
      <table>
        <thead>
          <tr>
            <th>Email</th>
            <th>Nombre</th>
            <th>Tenants y roles</th>
            <th>Acciones</th>
          </tr>
        </thead>
        <tbody>
          {items.map((user) => (
            <tr key={user.id}>
              <td>
                {user.email}
                {currentEmail?.toLowerCase() === user.email.toLowerCase() ? (
                  <small className="directory-self"> (vos)</small>
                ) : null}
              </td>
              <td>{user.display_name || "—"}</td>
              <td>
                {user.memberships
                  .map(
                    (membership) =>
                      `${membership.tenant_name}: ${membership.role}`,
                  )
                  .join(", ") || "—"}
              </td>
              <td>
                <LifecycleActions
                  busy={busy}
                  id={user.id}
                  onAction={onLifecycle}
                  state={state}
                />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {items.length === 0 ? (
        <div className="platform-access__empty">No hay usuarios en esta vista.</div>
      ) : null}
    </div>
  );
}

function TenantsTable({
  busy,
  items,
  onEdit,
  onLifecycle,
  state,
}: {
  busy?: string;
  items: AdminTenant[];
  onEdit: (tenant: AdminTenant) => void;
  onLifecycle: (id: string, action: LifecycleAction) => void;
  state: ResourceState;
}) {
  return (
    <div className="directory-table-wrap">
      <table>
        <thead>
          <tr>
            <th>Tenant</th>
            <th>Slug</th>
            <th>Estado</th>
            <th>Acciones</th>
          </tr>
        </thead>
        <tbody>
          {items.map((tenant) => (
            <tr key={tenant.id}>
              <td>{tenant.name}</td>
              <td>{tenant.slug}</td>
              <td>{tenant.status}</td>
              <td>
                <div className="directory-row-actions">
                  {state === "active" ? (
                    <button
                      disabled={Boolean(busy)}
                      onClick={() => onEdit(tenant)}
                      type="button"
                    >
                      Editar
                    </button>
                  ) : null}
                  <LifecycleActions
                    busy={busy}
                    id={tenant.id}
                    onAction={onLifecycle}
                    state={state}
                  />
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {items.length === 0 ? (
        <div className="platform-access__empty">No hay tenants en esta vista.</div>
      ) : null}
    </div>
  );
}

function LifecycleActions({
  busy,
  id,
  onAction,
  state,
}: {
  busy?: string;
  id: string;
  onAction: (id: string, action: LifecycleAction) => void;
  state: ResourceState;
}) {
  const disabled = Boolean(busy);
  if (state === "active") {
    return (
      <div className="directory-row-actions">
        <button disabled={disabled} onClick={() => onAction(id, "archive")} type="button">
          Archivar
        </button>
        <button
          className="is-danger"
          disabled={disabled}
          onClick={() => onAction(id, "trash")}
          type="button"
        >
          Papelera
        </button>
      </div>
    );
  }
  if (state === "archived") {
    return (
      <div className="directory-row-actions">
        <button
          disabled={disabled}
          onClick={() => onAction(id, "unarchive")}
          type="button"
        >
          Desarchivar
        </button>
      </div>
    );
  }
  return (
    <div className="directory-row-actions">
      <button disabled={disabled} onClick={() => onAction(id, "restore")} type="button">
        Restaurar
      </button>
      <button
        className="is-danger"
        disabled={disabled}
        onClick={() => onAction(id, "purge")}
        type="button"
      >
        Eliminar definitivamente
      </button>
    </div>
  );
}

function InvitationsTable({
  busy,
  items,
  onResend,
  onRevoke,
}: {
  busy?: string;
  items: Invitation[];
  onResend: (id: string) => void;
  onRevoke: (id: string) => void;
}) {
  return (
    <div className="directory-table-wrap">
      <table>
        <thead>
          <tr>
            <th>Email</th>
            <th>Rol</th>
            <th>Estado</th>
            <th>Vence</th>
            <th>Acciones</th>
          </tr>
        </thead>
        <tbody>
          {items.map((invitation) => (
            <tr key={invitation.id}>
              <td>{invitation.email}</td>
              <td>{invitation.role}</td>
              <td>{invitation.status}</td>
              <td>{new Date(invitation.expires_at).toLocaleDateString("es-AR")}</td>
              <td>
                <div className="directory-row-actions">
                  <button
                    disabled={Boolean(busy)}
                    onClick={() => onResend(invitation.id)}
                    type="button"
                  >
                    Reenviar
                  </button>
                  {invitation.status === "pending" ? (
                    <button
                      className="is-danger"
                      disabled={Boolean(busy)}
                      onClick={() => onRevoke(invitation.id)}
                      type="button"
                    >
                      Revocar
                    </button>
                  ) : null}
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {items.length === 0 ? (
        <div className="platform-access__empty">No hay invitaciones.</div>
      ) : null}
    </div>
  );
}
