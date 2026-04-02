import { useCallback, useEffect, useState } from 'react';
import {
  assignRbacRole,
  getUserEffectivePermissions,
  listOrgMembers,
  listRbacRoles,
  removeRbacRoleAssignment,
  type OrgMemberRow,
  type RbacRoleSummary,
} from '../lib/api';
import { formatFetchErrorForUser } from '../lib/formatFetchError';

function memberLabel(m: OrgMemberRow): string {
  const name = m.user?.name?.trim();
  const email = m.user?.email?.trim();
  if (name && email) return `${name} (${email})`;
  return name || email || m.user_id;
}

export function AdminRbacSection({ orgId }: { orgId: string }) {
  const [members, setMembers] = useState<OrgMemberRow[]>([]);
  const [roles, setRoles] = useState<RbacRoleSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [selectedRoleId, setSelectedRoleId] = useState('');
  const [selectedUserId, setSelectedUserId] = useState('');
  const [permUserId, setPermUserId] = useState<string | null>(null);
  const [permLines, setPermLines] = useState<string>('');

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const [mRes, rRes] = await Promise.all([listOrgMembers(orgId), listRbacRoles()]);
      setMembers(mRes.items ?? []);
      setRoles(rRes.items ?? []);
      setError('');
    } catch (err) {
      setError(formatFetchErrorForUser(err, 'No se pudieron cargar miembros o roles.'));
    } finally {
      setLoading(false);
    }
  }, [orgId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  async function onShowPermissions(userId: string): Promise<void> {
    setBusy(true);
    try {
      const { permissions } = await getUserEffectivePermissions(userId);
      const lines: string[] = [];
      const keys = Object.keys(permissions ?? {}).sort();
      for (const resource of keys) {
        const actions = permissions[resource] ?? [];
        lines.push(`${resource}: ${actions.join(', ')}`);
      }
      setPermUserId(userId);
      setPermLines(lines.length ? lines.join('\n') : '(sin permisos efectivos)');
      setError('');
    } catch (err) {
      setError(formatFetchErrorForUser(err, 'No se pudieron leer los permisos.'));
    } finally {
      setBusy(false);
    }
  }

  async function handleAssign(): Promise<void> {
    if (!selectedRoleId || !selectedUserId) {
      setError('Elegí un rol y un miembro.');
      return;
    }
    setBusy(true);
    try {
      await assignRbacRole(selectedRoleId, selectedUserId);
      setError('');
      await reload();
    } catch (err) {
      setError(formatFetchErrorForUser(err, 'No se pudo asignar el rol.'));
    } finally {
      setBusy(false);
    }
  }

  async function handleRemove(): Promise<void> {
    if (!selectedRoleId || !selectedUserId) {
      setError('Elegí un rol y un miembro para quitar.');
      return;
    }
    setBusy(true);
    try {
      await removeRbacRoleAssignment(selectedRoleId, selectedUserId);
      setError('');
      await reload();
    } catch (err) {
      setError(formatFetchErrorForUser(err, 'No se pudo quitar la asignación.'));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="card admin-settings-section">
      <div className="card-header">
        <h2>Miembros y roles (RBAC)</h2>
        <span className="badge badge-neutral">Solo administradores de consola</span>
      </div>
      <p className="admin-settings-hint">
        Asigná roles personalizados del catálogo <code>/v1/roles</code> a usuarios de la organización y consultá permisos
        efectivos.
      </p>
      {error ? <p className="form-error">{error}</p> : null}
      {loading ? (
        <p className="text-secondary">Cargando…</p>
      ) : (
        <>
          <div className="admin-activity-wrap">
            <table className="admin-activity-table">
              <thead>
                <tr>
                  <th>Miembro</th>
                  <th>User ID</th>
                  <th>Rol org.</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {members.map((m) => (
                  <tr key={m.id}>
                    <td>{memberLabel(m)}</td>
                    <td className="admin-activity-id">
                      <code className="admin-code">{m.user_id}</code>
                    </td>
                    <td>
                      <code className="admin-code">{m.role ?? '—'}</code>
                    </td>
                    <td>
                      <button
                        type="button"
                        className="btn-sm btn-secondary"
                        disabled={busy}
                        onClick={() => {
                          void onShowPermissions(m.user_id);
                        }}
                      >
                        Permisos efectivos
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {permUserId ? (
            <div className="admin-settings-section admin-rbac-block-mt">
              <h3>Permisos efectivos</h3>
              <p className="text-secondary">
                Usuario: <code className="admin-code">{permUserId}</code>
              </p>
              <pre className="admin-textarea admin-pre-permissions">
                {permLines}
              </pre>
              <button type="button" className="btn-sm btn-secondary" onClick={() => setPermUserId(null)}>
                Cerrar
              </button>
            </div>
          ) : null}

          <form
            className="admin-settings-grid admin-rbac-assign-form"
            onSubmit={(e) => {
              e.preventDefault();
              void handleAssign();
            }}
          >
            <div className="form-group">
              <label htmlFor="rbac-role">Rol (catálogo)</label>
              <select
                id="rbac-role"
                value={selectedRoleId}
                onChange={(e) => setSelectedRoleId(e.target.value)}
                disabled={busy}
              >
                <option value="">— Elegir —</option>
                {roles.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.name}
                  </option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label htmlFor="rbac-user">Miembro</label>
              <select
                id="rbac-user"
                value={selectedUserId}
                onChange={(e) => setSelectedUserId(e.target.value)}
                disabled={busy}
              >
                <option value="">— Elegir —</option>
                {members.map((m) => (
                  <option key={m.id} value={m.user_id}>
                    {memberLabel(m)}
                  </option>
                ))}
              </select>
            </div>
            <div className="form-group admin-settings-toolbar-bottom admin-rbac-form-actions">
              <button type="submit" className="btn-primary btn-sm" disabled={busy}>
                Asignar rol
              </button>
              <button type="button" className="btn-danger btn-sm" disabled={busy} onClick={() => void handleRemove()}>
                Quitar asignación
              </button>
            </div>
          </form>
        </>
      )}
    </section>
  );
}
