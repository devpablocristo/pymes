import {
  LifecycleActionToolbar,
  type LifecycleBulkAction,
  type LifecycleView,
} from "@devpablocristo/platform-lifecycle";
import { useEffect, useState, type FormEvent } from "react";
import type { components } from "../../api/schema.generated";
import { createIdempotencyKey } from "../../api/idempotency";
import { useProductApi } from "../../api/ProductApiContext";

type Tenant = components["schemas"]["AdminTenant"];
type TenantList = components["schemas"]["AdminTenantList"];

const stateForView: Record<LifecycleView, "active" | "archived" | "trashed"> = {
  active: "active",
  archived: "archived",
  trash: "trashed",
};

export function AdminTenantsPage() {
  const api = useProductApi();
  const [items, setItems] = useState<Tenant[]>([]);
  const [selected, setSelected] = useState<string[]>([]);
  const [view, setView] = useState<LifecycleView>("active");
  const [createOpen, setCreateOpen] = useState(true);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [adminEmail, setAdminEmail] = useState("");
  const [editing, setEditing] = useState<Tenant>();
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
    api
      .request<TenantList>(`/api/v1/admin/tenants?${search}`, {
        signal: controller.signal,
        skipJSONContentType: true,
      })
      .then((response) => {
        setItems(response.items);
        setSelected([]);
      })
      .catch((cause: unknown) => {
        if (!controller.signal.aborted) {
          setError(cause instanceof Error ? cause.message : "No pudimos cargar los tenants.");
        }
      });
    return () => controller.abort();
  }, [api, query, revision, view]);

  async function command(
    operation: string,
    path: string,
    method: "POST" | "PATCH" | "DELETE",
    body?: unknown,
  ) {
    setBusy(operation);
    setError(undefined);
    setNotice(undefined);
    try {
      await api.request(path, {
        method,
        headers: { "Idempotency-Key": createIdempotencyKey(operation) },
        ...(body === undefined ? {} : { body: JSON.stringify(body) }),
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
    const succeeded = await command("admin-tenant-create", "/api/v1/admin/tenants", "POST", {
      name: name.trim(),
      slug: slug.trim().toLowerCase(),
      admin_email: adminEmail.trim().toLowerCase(),
    });
    if (succeeded) {
      setName("");
      setSlug("");
      setAdminEmail("");
    }
  }

  async function updateTenant(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!editing) return;
    const succeeded = await command(
      `admin-tenant-update-${editing.id}`,
      `/api/v1/admin/tenants/${editing.id}`,
      "PATCH",
      { name: editing.name.trim(), slug: editing.slug.trim().toLowerCase() },
    );
    if (succeeded) setEditing(undefined);
  }

  async function lifecycleAction(action: LifecycleBulkAction) {
    const endpoint =
      action === "restore" && view === "archived" ? "unarchive" : action;
    const method = endpoint === "purge" ? "DELETE" : "POST";
    const targets = [...selected];
    setBusy(`admin-tenant-${endpoint}`);
    setError(undefined);
    setNotice(undefined);
    try {
      await Promise.all(
        targets.map((id) =>
          api.request(`/api/v1/admin/tenants/${id}/${endpoint}`, {
            method,
            headers: {
              "Idempotency-Key": createIdempotencyKey(`admin-tenant-${endpoint}-${id}`),
            },
            body: JSON.stringify({}),
          }),
        ),
      );
      setSelected([]);
      setNotice(targets.length === 1 ? "Cambio guardado." : `${targets.length} tenants actualizados.`);
      setRevision((value) => value + 1);
    } catch (cause: unknown) {
      setError(cause instanceof Error ? cause.message : "No pudimos aplicar el cambio.");
    } finally {
      setBusy(undefined);
    }
  }

  const selectedTenant = selected.length === 1
    ? items.find((tenant) => tenant.id === selected[0])
    : undefined;

  return (
    <div className="settings-page">
      <header className="page-topbar">
        <div>
          <h1>Tenants</h1>
          <small>Administración global</small>
        </div>
      </header>
      <div className="settings-canvas admin-canvas">
        <div className="settings-heading">
          <div>
            <h2>Organizaciones de Pymes</h2>
            <p>Lifecycle canónico de Platform: activos, archivados y papelera.</p>
          </div>
          <span className="settings-count">{items.length}</span>
        </div>

        <div className="admin-lifecycle-tabs" role="tablist" aria-label="Estado de tenants">
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
          onEdit={() => selectedTenant && setEditing(selectedTenant)}
          selectedCount={selected.length}
          view={view}
          labels={{
            newButton: "Nuevo tenant",
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
          <form className="admin-create-form" onSubmit={(event) => void createTenant(event)}>
            <label>
              <span>Nombre</span>
              <input required maxLength={120} value={name} onChange={(event) => setName(event.target.value)} />
            </label>
            <label>
              <span>Slug</span>
              <input
                required
                minLength={2}
                maxLength={63}
                pattern="[a-z0-9]+(?:-[a-z0-9]+)*"
                value={slug}
                onChange={(event) => setSlug(event.target.value.toLowerCase())}
              />
            </label>
            <label>
              <span>Administrador inicial</span>
              <input required type="email" value={adminEmail} onChange={(event) => setAdminEmail(event.target.value)} />
            </label>
            <button disabled={Boolean(busy)} type="submit">
              {busy === "admin-tenant-create" ? "Creando…" : "Crear tenant"}
            </button>
          </form>
        ) : null}

        <div className="admin-toolbar">
          <input
            type="search"
            placeholder="Buscar por nombre o slug"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>

        <div className="settings-list">
          {items.map((tenant) => (
            <article className="settings-row admin-row" key={tenant.id}>
              <input
                aria-label={`Seleccionar ${tenant.name}`}
                checked={selected.includes(tenant.id)}
                onChange={(event) =>
                  setSelected((current) =>
                    event.target.checked
                      ? [...current, tenant.id]
                      : current.filter((id) => id !== tenant.id),
                  )
                }
                type="checkbox"
              />
              <div className="settings-row__body">
                <div className="settings-row__title">
                  <strong>{tenant.name}</strong>
                  <span className={`status-pill status-pill--${tenant.lifecycle_state}`}>
                    {tenant.lifecycle_state}
                  </span>
                </div>
                <span>{tenant.slug}</span>
                <small>Estado operativo: {tenant.status} · Sincronización: {tenant.sync_status}</small>
              </div>
            </article>
          ))}
        </div>
      </div>

      {editing ? (
        <div className="admin-dialog-backdrop" role="presentation">
          <form className="admin-dialog" onSubmit={(event) => void updateTenant(event)}>
            <h2>Editar tenant</h2>
            <label>
              <span>Nombre</span>
              <input required value={editing.name} onChange={(event) => setEditing({ ...editing, name: event.target.value })} />
            </label>
            <label>
              <span>Slug</span>
              <input
                required
                pattern="[a-z0-9]+(?:-[a-z0-9]+)*"
                value={editing.slug}
                onChange={(event) => setEditing({ ...editing, slug: event.target.value.toLowerCase() })}
              />
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
