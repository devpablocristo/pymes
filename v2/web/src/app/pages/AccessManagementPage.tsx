import {
  matchesAccessSearch,
  normalizeAccessSearch,
  type AccessTab,
} from "@devpablocristo/platform-access-management";
import "@devpablocristo/platform-access-management/styles.css";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import type { components } from "../../api/schema.generated";
import { createIdempotencyKey } from "../../api/idempotency";
import { useProductApi } from "../../api/ProductApiContext";
import { useProductAuth } from "../../auth/AuthContext";
import {
  EntityLifecycleBulkToolbar,
  EntityLifecycleTabs,
  EntitySelectionToolbar,
  toApiLifecycleState,
  type EntityLifecycleAction,
  type EntityLifecycleState,
} from "../components/EntityLifecycle";
import { SectionHeader } from "../shell/SectionChrome";

type AdminTenant = components["schemas"]["AdminTenant"];
type AdminTenantList = components["schemas"]["AdminTenantList"];
type AdminUser = components["schemas"]["AdminUser"];
type AdminUserList = components["schemas"]["AdminUserList"];
type Invitation = components["schemas"]["Invitation"];
type InvitationList = components["schemas"]["InvitationList"];

const tabLabels: Record<AccessTab, string> = {
  users: "Usuarios",
  tenants: "Tenants",
  invitations: "Invitaciones",
};

export function AccessManagementPage() {
  const api = useProductApi();
  const auth = useProductAuth();
  const [activeTab, setActiveTab] = useState<AccessTab>("users");
  const [resourceStates, setResourceStates] = useState<
    Record<"users" | "tenants", EntityLifecycleState>
  >({
    users: "active",
    tenants: "active",
  });
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [tenants, setTenants] = useState<AdminTenant[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [selectedIds, setSelectedIds] = useState<
    Record<"users" | "tenants", string[]>
  >({ users: [], tenants: [] });
  const [selectedInvitationIds, setSelectedInvitationIds] = useState<string[]>([]);
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
  const selectedResourceIds =
    activeTab === "invitations" ? [] : selectedIds[activeTab];

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(undefined);

    const request =
      activeTab === "users"
        ? api
            .request<AdminUserList>(
              `/api/v1/admin/users?limit=100&lifecycle_state=${toApiLifecycleState(resourceStates.users)}`,
              { signal: controller.signal, skipJSONContentType: true },
            )
            .then((response) => setUsers(response.items))
        : activeTab === "tenants"
          ? api
              .request<AdminTenantList>(
                `/api/v1/admin/tenants?limit=100&lifecycle_state=${toApiLifecycleState(resourceStates.tenants)}`,
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

  async function applyLifecycle(action: EntityLifecycleAction) {
    if (activeTab === "invitations") return;
    const resource = activeTab;
    const ids = selectedIds[resource];
    if (ids.length === 0) return;
    if (
      action === "purge" &&
      !window.confirm("Esta acción elimina el registro definitivamente. ¿Continuar?")
    ) {
      return;
    }
    setBusy(`bulk-${resource}-${action}`);
    setError(undefined);
    setNotice(undefined);
    try {
      for (const resourceId of ids) {
        await api.request(
          `/api/v1/admin/${resource}/${encodeURIComponent(resourceId)}/${action}`,
          {
            method: action === "purge" ? "DELETE" : "POST",
            headers: {
              "Idempotency-Key": createIdempotencyKey(
                `admin-${resource}-${action}-${resourceId}`,
              ),
            },
            body: JSON.stringify({}),
          },
        );
      }
      setSelectedIds((current) => ({ ...current, [resource]: [] }));
      setNotice("Cambio guardado.");
      setRevision((current) => current + 1);
    } catch (cause: unknown) {
      setError(
        cause instanceof Error ? cause.message : "No pudimos aplicar el cambio.",
      );
    } finally {
      setBusy(undefined);
    }
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
    setSelectedIds({ users: [], tenants: [] });
    setSelectedInvitationIds([]);
    setQuery("");
    setError(undefined);
    setNotice(undefined);
  }

  function toggleSelected(
    resource: "users" | "tenants",
    id: string,
    checked: boolean,
  ) {
    setSelectedIds((current) => ({
      ...current,
      [resource]: checked
        ? Array.from(new Set([...current[resource], id]))
        : current[resource].filter((candidate) => candidate !== id),
    }));
  }

  function toggleInvitation(id: string, checked: boolean) {
    setSelectedInvitationIds((current) =>
      checked
        ? Array.from(new Set([...current, id]))
        : current.filter((candidate) => candidate !== id),
    );
  }

  async function applyInvitationAction(action: "resend" | "revoke") {
    if (selectedInvitationIds.length === 0) return;
    setBusy(`bulk-invitations-${action}`);
    setError(undefined);
    setNotice(undefined);
    try {
      for (const id of selectedInvitationIds) {
        await api.request(
          `/api/v1/team/invitations/${encodeURIComponent(id)}/${action}`,
          {
            method: "POST",
            headers: {
              "Idempotency-Key": createIdempotencyKey(
                `team-invitation-${action}-${id}`,
              ),
            },
            body: JSON.stringify({}),
          },
        );
      }
      setSelectedInvitationIds([]);
      setNotice("Cambio guardado.");
      setRevision((current) => current + 1);
    } catch (cause: unknown) {
      setError(
        cause instanceof Error ? cause.message : "No pudimos aplicar el cambio.",
      );
    } finally {
      setBusy(undefined);
    }
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
                  <>
                    <EntityLifecycleTabs
                      label={`Estado de ${tabLabels[activeTab].toLowerCase()}`}
                      state={selectedResourceState}
                      onChange={(state) => {
                        setResourceStates((current) => ({
                          ...current,
                          [activeTab]: state,
                        }));
                        setSelectedIds((current) => ({
                          ...current,
                          [activeTab]: [],
                        }));
                      }}
                    />
                    <EntityLifecycleBulkToolbar
                      busy={Boolean(busy)}
                      editOpen={Boolean(editingTenant)}
                      onAction={(action) => void applyLifecycle(action)}
                      onClear={() =>
                        setSelectedIds((current) => ({
                          ...current,
                          [activeTab]: [],
                        }))
                      }
                      onEdit={
                        activeTab === "tenants" && selectedIds.tenants.length === 1
                          ? () =>
                              setEditingTenant(
                                tenants.find(
                                  (tenant) => tenant.id === selectedIds.tenants[0],
                                ),
                              )
                          : activeTab === "tenants"
                            ? () => undefined
                            : undefined
                      }
                      selectedCount={selectedResourceIds.length}
                      state={selectedResourceState}
                    />
                  </>
                ) : null}
                {activeTab === "invitations" ? (
                  <EntitySelectionToolbar
                    actions={[
                      {
                        id: "resend",
                        label: "Reenviar",
                        onClick: () => void applyInvitationAction("resend"),
                      },
                      {
                        id: "revoke",
                        label: "Revocar",
                        danger: true,
                        disabled: selectedInvitationIds.some(
                          (id) =>
                            invitations.find((invitation) => invitation.id === id)
                              ?.status !== "pending",
                        ),
                        onClick: () => void applyInvitationAction("revoke"),
                      },
                    ]}
                    busy={Boolean(busy)}
                    createLabel="Nueva invitación"
                    onClear={() => setSelectedInvitationIds([])}
                    onCreate={
                      activeOrganization ? () => setInvitationOpen(true) : undefined
                    }
                    selectedCount={selectedInvitationIds.length}
                  />
                ) : null}
              </div>
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
                    currentEmail={auth.user?.email}
                    items={visibleUsers}
                    selectedIds={selectedIds.users}
                    onToggle={(id, checked) =>
                      toggleSelected("users", id, checked)
                    }
                  />
                ) : null}
                {activeTab === "tenants" ? (
                  <TenantsTable
                    items={visibleTenants}
                    selectedIds={selectedIds.tenants}
                    onToggle={(id, checked) =>
                      toggleSelected("tenants", id, checked)
                    }
                  />
                ) : null}
                {activeTab === "invitations" ? (
                  <InvitationsTable
                    items={visibleInvitations}
                    selectedIds={selectedInvitationIds}
                    onToggle={toggleInvitation}
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
  currentEmail,
  items,
  onToggle,
  selectedIds,
}: {
  currentEmail?: string;
  items: AdminUser[];
  onToggle: (id: string, checked: boolean) => void;
  selectedIds: string[];
}) {
  return (
    <div className="directory-table-wrap">
      <table>
        <thead>
          <tr>
            <th className="entity-select-cell" />
            <th>Email</th>
            <th>Nombre</th>
            <th>Tenants y roles</th>
          </tr>
        </thead>
        <tbody>
          {items.map((user) => (
            <tr key={user.id}>
              <td className="entity-select-cell">
                <input
                  aria-label={`Seleccionar ${user.display_name || user.email}`}
                  checked={selectedIds.includes(user.id)}
                  onChange={(event) => onToggle(user.id, event.currentTarget.checked)}
                  type="checkbox"
                />
              </td>
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
  items,
  onToggle,
  selectedIds,
}: {
  items: AdminTenant[];
  onToggle: (id: string, checked: boolean) => void;
  selectedIds: string[];
}) {
  return (
    <div className="directory-table-wrap">
      <table>
        <thead>
          <tr>
            <th className="entity-select-cell" />
            <th>Tenant</th>
            <th>Slug</th>
            <th>Estado</th>
          </tr>
        </thead>
        <tbody>
          {items.map((tenant) => (
            <tr key={tenant.id}>
              <td className="entity-select-cell">
                <input
                  aria-label={`Seleccionar ${tenant.name}`}
                  checked={selectedIds.includes(tenant.id)}
                  onChange={(event) =>
                    onToggle(tenant.id, event.currentTarget.checked)
                  }
                  type="checkbox"
                />
              </td>
              <td>{tenant.name}</td>
              <td>{tenant.slug}</td>
              <td>{tenant.status}</td>
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

function InvitationsTable({
  items,
  onToggle,
  selectedIds,
}: {
  items: Invitation[];
  onToggle: (id: string, checked: boolean) => void;
  selectedIds: string[];
}) {
  return (
    <div className="directory-table-wrap">
      <table>
        <thead>
          <tr>
            <th className="entity-select-cell" />
            <th>Email</th>
            <th>Rol</th>
            <th>Estado</th>
            <th>Vence</th>
          </tr>
        </thead>
        <tbody>
          {items.map((invitation) => (
            <tr key={invitation.id}>
              <td className="entity-select-cell">
                <input
                  aria-label={`Seleccionar ${invitation.email}`}
                  checked={selectedIds.includes(invitation.id)}
                  onChange={(event) =>
                    onToggle(invitation.id, event.currentTarget.checked)
                  }
                  type="checkbox"
                />
              </td>
              <td>{invitation.email}</td>
              <td>{invitation.role}</td>
              <td>{invitation.status}</td>
              <td>{new Date(invitation.expires_at).toLocaleDateString("es-AR")}</td>
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
