import { useState, type FormEvent } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createTenantInvite,
  listTenantInvites,
  listTenantMembers,
  resendTenantInvite,
  revokeTenantInvite,
  type TenantInvitation,
  type TenantMemberRow,
} from '../lib/api';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { queryKeys } from '../lib/queryKeys';

type SettingsTeamSectionProps = {
  tenantId: string;
  membershipRole?: string;
};

function memberLabel(member: TenantMemberRow): string {
  const name = member.user?.name?.trim();
  const email = member.user?.email?.trim();
  if (name && email) return `${name} (${email})`;
  return name || email || member.user_id;
}

function roleLabel(role: string | undefined): string {
  switch (role) {
    case 'owner':
      return 'Owner';
    case 'admin':
      return 'Admin';
    case 'member':
      return 'Miembro';
    default:
      return role || '—';
  }
}

function statusLabel(status: TenantInvitation['status']): string {
  switch (status) {
    case 'pending':
      return 'Pendiente';
    case 'accepted':
      return 'Aceptada';
    case 'revoked':
      return 'Revocada';
    case 'expired':
      return 'Expirada';
    default:
      return status;
  }
}

export function SettingsTeamSection({ tenantId, membershipRole }: SettingsTeamSectionProps) {
  const queryClient = useQueryClient();
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [inviteOpen, setInviteOpen] = useState(false);
  const [inviteEmail, setInviteEmail] = useState('');
  const [inviteRole, setInviteRole] = useState<'member' | 'admin'>('member');
  const canManageInvites = membershipRole === 'owner';

  const membersQuery = useQuery({
    queryKey: queryKeys.rbac.members(tenantId),
    queryFn: () => listTenantMembers(tenantId),
    enabled: Boolean(tenantId),
  });
  const invitesQuery = useQuery({
    queryKey: queryKeys.rbac.invites(tenantId),
    queryFn: () => listTenantInvites(tenantId),
    enabled: Boolean(tenantId) && canManageInvites,
  });

  const inviteMutation = useMutation({
    mutationFn: ({ email, role }: { email: string; role: string }) => createTenantInvite(tenantId, { email, role }),
    onSuccess: async () => {
      setInviteEmail('');
      setInviteRole('member');
      setInviteOpen(false);
      setSuccess('Invitación enviada.');
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.rbac.members(tenantId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.rbac.invites(tenantId) }),
      ]);
    },
  });
  const revokeMutation = useMutation({
    mutationFn: (inviteId: string) => revokeTenantInvite(inviteId),
    onSuccess: async () => {
      setSuccess('Invitación revocada.');
      await queryClient.invalidateQueries({ queryKey: queryKeys.rbac.invites(tenantId) });
    },
  });
  const resendMutation = useMutation({
    mutationFn: (inviteId: string) => resendTenantInvite(inviteId),
    onSuccess: async () => {
      setSuccess('Invitación reenviada.');
      await queryClient.invalidateQueries({ queryKey: queryKeys.rbac.invites(tenantId) });
    },
  });

  const members = membersQuery.data?.items ?? [];
  const invites = invitesQuery.data?.items ?? [];
  const busy = inviteMutation.isPending || revokeMutation.isPending || resendMutation.isPending;
  const loadError = membersQuery.error || invitesQuery.error;

  async function handleInviteSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    const email = inviteEmail.trim().toLowerCase();
    if (!email) {
      setError('Ingresá un email para invitar.');
      setSuccess('');
      return;
    }
    try {
      setError('');
      setSuccess('');
      await inviteMutation.mutateAsync({ email, role: inviteRole });
    } catch (err) {
      setError(formatFetchErrorForUser(err, 'No se pudo enviar la invitación.'));
    }
  }

  return (
    <section className="card admin-settings-section">
      <div className="card-header">
        <div>
          <h2>Equipo</h2>
          <p className="admin-settings-hint u-m-0">Miembros del tenant actual e invitaciones pendientes.</p>
        </div>
        {canManageInvites ? (
          <button
            type="button"
            className="btn-primary btn-sm"
            onClick={() => {
              setInviteOpen((open) => !open);
              setError('');
              setSuccess('');
            }}
          >
            Invitar usuario
          </button>
        ) : (
          <span className="badge badge-neutral">Solo owner invita</span>
        )}
      </div>

      {error ? <p className="form-error">{error}</p> : null}
      {success ? <p className="form-success">{success}</p> : null}
      {!error && loadError ? (
        <p className="form-error">{formatFetchErrorForUser(loadError, 'No se pudo cargar el equipo.')}</p>
      ) : null}

      {inviteOpen && canManageInvites ? (
        <form className="admin-settings-grid admin-rbac-assign-form" onSubmit={(event) => void handleInviteSubmit(event)}>
          <div className="form-group">
            <label htmlFor="settings-team-invite-email">Email</label>
            <input
              id="settings-team-invite-email"
              type="email"
              value={inviteEmail}
              onChange={(event) => setInviteEmail(event.target.value)}
              placeholder="persona@empresa.com"
              disabled={busy}
              autoFocus
              required
            />
          </div>
          <div className="form-group">
            <label htmlFor="settings-team-invite-role">Rol en el tenant</label>
            <select
              id="settings-team-invite-role"
              value={inviteRole}
              onChange={(event) => setInviteRole(event.target.value === 'admin' ? 'admin' : 'member')}
              disabled={busy}
            >
              <option value="member">Miembro</option>
              <option value="admin">Admin</option>
            </select>
          </div>
          <div className="form-group admin-settings-toolbar-bottom admin-rbac-form-actions">
            <button type="submit" className="btn-primary btn-sm" disabled={busy}>
              {inviteMutation.isPending ? 'Enviando…' : 'Enviar invitación'}
            </button>
            <button type="button" className="btn-secondary btn-sm" disabled={busy} onClick={() => setInviteOpen(false)}>
              Cancelar
            </button>
          </div>
        </form>
      ) : null}

      {membersQuery.isLoading ? (
        <p className="text-secondary">Cargando miembros…</p>
      ) : (
        <div className="admin-activity-wrap">
          <table className="admin-activity-table">
            <thead>
              <tr>
                <th>Miembro</th>
                <th>Rol</th>
                <th>Estado</th>
              </tr>
            </thead>
            <tbody>
              {members.map((member) => (
                <tr key={member.id}>
                  <td>{memberLabel(member)}</td>
                  <td>
                    <code className="admin-code">{roleLabel(member.role)}</code>
                  </td>
                  <td>
                    <code className="admin-code">{member.status ?? 'active'}</code>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {canManageInvites ? (
        <div className="admin-settings-section admin-rbac-block-mt">
          <h3>Invitaciones</h3>
          {invitesQuery.isLoading ? <p className="text-secondary">Cargando invitaciones…</p> : null}
          {!invitesQuery.isLoading && invites.length === 0 ? (
            <p className="text-secondary">No hay invitaciones pendientes o recientes.</p>
          ) : null}
          {invites.length > 0 ? (
            <div className="admin-activity-wrap">
              <table className="admin-activity-table">
                <thead>
                  <tr>
                    <th>Email</th>
                    <th>Rol</th>
                    <th>Estado</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {invites.map((invite) => (
                    <tr key={invite.id}>
                      <td>{invite.email}</td>
                      <td>
                        <code className="admin-code">{roleLabel(invite.role)}</code>
                      </td>
                      <td>
                        <code className="admin-code">{statusLabel(invite.status)}</code>
                      </td>
                      <td>
                        {invite.status === 'pending' ? (
                          <>
                            <button
                              type="button"
                              className="btn-sm btn-secondary"
                              disabled={busy}
                              onClick={() => void resendMutation.mutateAsync(invite.id)}
                            >
                              Reenviar
                            </button>{' '}
                            <button
                              type="button"
                              className="btn-sm btn-danger"
                              disabled={busy}
                              onClick={() => void revokeMutation.mutateAsync(invite.id)}
                            >
                              Revocar
                            </button>
                          </>
                        ) : null}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}
