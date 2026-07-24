import { useEffect, useMemo, useState, type FormEvent } from "react";
import type { components } from "../../api/schema.generated";
import { createIdempotencyKey } from "../../api/idempotency";
import { useProductApi } from "../../api/ProductApiContext";
import { SectionHeader, SectionSearch } from "../shell/SectionChrome";
import { AdminDirectoryTabs } from "./AdminDirectoryTabs";

type Tenant = components["schemas"]["AdminTenant"];
type TenantList = components["schemas"]["AdminTenantList"];
type View = "active" | "archived" | "trashed";

const viewLabels: Record<View, string> = {
  active: "Activos",
  archived: "Archivados",
  trashed: "Papelera",
};

export function AdminTenantsPage() {
  const api = useProductApi();
  const [items, setItems] = useState<Tenant[]>([]);
  const [view, setView] = useState<View>("active");
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [adminEmail, setAdminEmail] = useState("");
  const [editing, setEditing] = useState<Tenant>();
  const [busy, setBusy] = useState<string>();
  const [error, setError] = useState<string>();
  const [notice, setNotice] = useState<string>();
  const [revision, setRevision] = useState(0);
  const [search, setSearch] = useState("");
  const visibleItems = useMemo(() => {
    const query = search.trim().toLocaleLowerCase("es");
    if (!query) return items;
    return items.filter((tenant) =>
      [tenant.name, tenant.slug, tenant.status].some((value) =>
        value.toLocaleLowerCase("es").includes(query),
      ),
    );
  }, [items, search]);

  useEffect(() => {
    const controller = new AbortController();
    api
      .request<TenantList>(
        `/api/v1/admin/tenants?limit=100&lifecycle_state=${view}`,
        { signal: controller.signal, skipJSONContentType: true },
      )
      .then((response) => setItems(response.items))
      .catch((cause: unknown) => {
        if (!controller.signal.aborted) {
          setError(cause instanceof Error ? cause.message : "No pudimos cargar los tenants.");
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

  async function createTenant(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const succeeded = await request("admin-tenant-create", "/api/v1/admin/tenants", "POST", {
      name: name.trim(),
      slug: slug.trim().toLowerCase(),
      admin_email: adminEmail.trim().toLowerCase(),
    });
    if (succeeded) {
      setName("");
      setSlug("");
      setAdminEmail("");
      setCreateOpen(false);
    }
  }

  async function updateTenant(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!editing) return;
    const succeeded = await request(
      `admin-tenant-update-${editing.id}`,
      `/api/v1/admin/tenants/${editing.id}`,
      "PATCH",
      { name: editing.name.trim(), slug: editing.slug.trim().toLowerCase() },
    );
    if (succeeded) setEditing(undefined);
  }

  async function lifecycle(tenant: Tenant, action: "archive" | "trash" | "unarchive" | "restore" | "purge") {
    await request(
      `admin-tenant-${action}-${tenant.id}`,
      `/api/v1/admin/tenants/${tenant.id}/${action}`,
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
              <h2>Tenants</h2>
              <div className="lifecycle-tabs" role="tablist" aria-label="Estado de tenants">
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
                label="Buscar tenants"
                placeholder="Buscar tenants…"
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
                  <th>Tenant</th>
                  <th>Slug</th>
                  <th>Estado</th>
                  <th>Acciones</th>
                </tr>
              </thead>
              <tbody>
                {visibleItems.map((tenant) => (
                  <tr key={tenant.id}>
                    <td>{tenant.name}</td>
                    <td>{tenant.slug}</td>
                    <td>{tenant.status}</td>
                    <td>
                      <div className="directory-row-actions">
                        {view === "active" ? (
                          <>
                            <button onClick={() => setEditing(tenant)} type="button">Editar</button>
                            <button onClick={() => void lifecycle(tenant, "archive")} type="button">Archivar</button>
                            <button className="is-danger" onClick={() => void lifecycle(tenant, "trash")} type="button">Papelera</button>
                          </>
                        ) : null}
                        {view === "archived" ? (
                          <button onClick={() => void lifecycle(tenant, "unarchive")} type="button">Restaurar</button>
                        ) : null}
                        {view === "trashed" ? (
                          <>
                            <button onClick={() => void lifecycle(tenant, "restore")} type="button">Restaurar</button>
                            <button className="is-danger" onClick={() => void lifecycle(tenant, "purge")} type="button">Eliminar</button>
                          </>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                ))}
                {visibleItems.length === 0 ? (
                  <tr>
                    <td className="directory-empty" colSpan={4}>
                      {items.length === 0 ? "No hay tenants en esta vista." : "No hay tenants que coincidan con la búsqueda."}
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
          <form className="admin-dialog" onSubmit={(event) => void createTenant(event)}>
            <h2>Crear tenant</h2>
            <label><span>Nombre</span><input required maxLength={120} value={name} onChange={(event) => setName(event.target.value)} /></label>
            <label><span>Slug</span><input required pattern="[a-z0-9]+(?:-[a-z0-9]+)*" value={slug} onChange={(event) => setSlug(event.target.value.toLowerCase())} /></label>
            <label><span>Administrador inicial</span><input required type="email" value={adminEmail} onChange={(event) => setAdminEmail(event.target.value)} /></label>
            <div className="admin-dialog__actions">
              <button type="button" onClick={() => setCreateOpen(false)}>Cancelar</button>
              <button type="submit" disabled={Boolean(busy)}>Crear tenant</button>
            </div>
          </form>
        </div>
      ) : null}

      {editing ? (
        <div className="admin-dialog-backdrop" role="presentation">
          <form className="admin-dialog" onSubmit={(event) => void updateTenant(event)}>
            <h2>Editar tenant</h2>
            <label><span>Nombre</span><input required value={editing.name} onChange={(event) => setEditing({ ...editing, name: event.target.value })} /></label>
            <label><span>Slug</span><input required pattern="[a-z0-9]+(?:-[a-z0-9]+)*" value={editing.slug} onChange={(event) => setEditing({ ...editing, slug: event.target.value.toLowerCase() })} /></label>
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
