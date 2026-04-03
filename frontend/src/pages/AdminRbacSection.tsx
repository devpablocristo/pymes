import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  assignRbacRole,
  getUserEffectivePermissions,
  listOrgMembers,
  listRbacRoles,
  removeRbacRoleAssignment,
  type OrgMemberRow,
} from '../lib/api';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { queryKeys } from '../lib/queryKeys';

function memberLabel(m: OrgMemberRow): string {
  const name = m.user?.name?.trim();
  const email = m.user?.email?.trim();
  if (name && email) return `${name} (${email})`;
  return name || email || m.user_id;
}

export function AdminRbacSection({ orgId }: { orgId: string }) {
  const [error, setError] = useState('');
  const [selectedRoleId, setSelectedRoleId] = useState('');
  const [selectedUserId, setSelectedUserId] = useState('');
  const [permUserId, setPermUserId] = useState<string | null>(null);
  const queryClient = useQueryClient();
  const membersQuery = useQuery({
    queryKey: queryKeys.rbac.members(orgId),
    queryFn: () => listOrgMembers(orgId),
  });
  const rolesQuery = useQuery({
    queryKey: queryKeys.rbac.roles,
    queryFn: listRbacRoles,
  });
  const permissionsQuery = useQuery({
    queryKey: queryKeys.rbac.permissions(permUserId ?? ''),
    queryFn: () => getUserEffectivePermissions(permUserId ?? ''),
    enabled: permUserId !== null,
  });
  const assignMutation = useMutation({
    mutationFn: ({ roleId, userId }: { roleId: string; userId: string }) => assignRbacRole(roleId, userId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.rbac.members(orgId) });
    },
  });
  const removeMutation = useMutation({
    mutationFn: ({ roleId, userId }: { roleId: string; userId: string }) => removeRbacRoleAssignment(roleId, userId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.rbac.members(orgId) });
    },
  });
  const members = membersQuery.data?.items ?? [];
  const roles = rolesQuery.data?.items ?? [];
  const loading = membersQuery.isLoading || rolesQuery.isLoading;
  const busy = assignMutation.isPending || removeMutation.isPending;
  const queryError = membersQuery.error || rolesQuery.error;
  const permLines = useMemo(() => {
    const permissions = permissionsQuery.data?.permissions ?? {};
    const lines: string[] = [];
    const keys = Object.keys(permissions).sort();
    for (const resource of keys) {
      const actions = permissions[resource] ?? [];
      lines.push(`${resource}: ${actions.join(', ')}`);
    }
    return lines.length ? lines.join('\n') : '(sin permisos efectivos)';
  }, [permissionsQuery.data]);

  async function onShowPermissions(userId: string): Promise<void> {
    try {
      setPermUserId(userId);
      setError('');
    } catch (err) {
      setError(formatFetchErrorForUser(err, 'No se pudieron leer los permisos.'));
    }
  }

  async function handleAssign(): Promise<void> {
    if (!selectedRoleId || !selectedUserId) {
      setError('Elegí un rol y un miembro.');
      return;
    }
    try {
      await assignMutation.mutateAsync({ roleId: selectedRoleId, userId: selectedUserId });
      setError('');
    } catch (err) {
      setError(formatFetchErrorForUser(err, 'No se pudo asignar el rol.'));
    }
  }

  async function handleRemove(): Promise<void> {
    if (!selectedRoleId || !selectedUserId) {
      setError('Elegí un rol y un miembro para quitar.');
      return;
    }
    try {
      await removeMutation.mutateAsync({ roleId: selectedRoleId, userId: selectedUserId });
      setError('');
    } catch (err) {
      setError(formatFetchErrorForUser(err, 'No se pudo quitar la asignación.'));
    }
  }

  return (
    <section className="card admin-settings-section">
      <div className="card-header">
        <h2>Miembros y roles (RBAC)</h2>
        <span className="badge badge-neutral">Solo administradores de consola</span>
      </div>
      <p className="admin-settings-hint">
        Asigná roles personalizados del catálogo <code>/v1/roles</code> a usuarios de la organización y consultá
        permisos efectivos.
      </p>
      {error ? <p className="form-error">{error}</p> : null}
      {!error && queryError ? (
        <p className="form-error">{formatFetchErrorForUser(queryError, 'No se pudieron cargar miembros o roles.')}</p>
      ) : null}
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
              {permissionsQuery.isLoading ? <p className="text-secondary">Cargando permisos…</p> : null}
              {permissionsQuery.error ? (
                <p className="form-error">
                  {formatFetchErrorForUser(permissionsQuery.error, 'No se pudieron leer los permisos.')}
                </p>
              ) : null}
              <pre className="admin-textarea admin-pre-permissions">{permLines}</pre>
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
