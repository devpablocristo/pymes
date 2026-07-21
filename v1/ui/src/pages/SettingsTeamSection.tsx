import { useState, type FormEvent } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createTenantInvite,
  listTenantInvites,
  listTenantMembers,
  removeTenantMember,
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

function memberNameParts(member: TenantMemberRow): { firstName: string; lastName: string; email: string } {
  const email = member.user?.email?.trim() || '—';
  const rawGivenName = member.user?.given_name?.trim() || '';
  const rawFamilyName = member.user?.family_name?.trim() || '';
  const givenName = rawGivenName && !isPlaceholderClerkName(rawGivenName) ? rawGivenName : '';
  const familyName = rawFamilyName && !isPlaceholderClerkName(rawFamilyName) ? rawFamilyName : '';
  if (givenName || familyName) {
    return { firstName: givenName || '—', lastName: familyName || '—', email };
  }

  const fullName = member.user?.name?.trim() || '';
  if (!fullName || isPlaceholderClerkName(fullName)) {
    return { firstName: '—', lastName: '—', email };
  }
  const [firstName, ...rest] = fullName.split(/\s+/);
  return { firstName: firstName || '—', lastName: rest.join(' ') || '—', email };
}

function isPlaceholderClerkName(name: string): boolean {
  const normalized = name.trim().toLowerCase();
  return normalized.endsWith('@users.clerk.placeholder') || normalized.startsWith('user_');
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
  const removeMemberMutation = useMutation({
    mutationFn: (userId: string) => removeTenantMember(tenantId, userId),
    onSuccess: async () => {
      setSuccess('Usuario eliminado del tenant.');
      await queryClient.invalidateQueries({ queryKey: queryKeys.rbac.members(tenantId) });
    },
  });

  const members = membersQuery.data?.items ?? [];
  const invites = (invitesQuery.data?.items ?? []).filter((invite) => invite.status === 'pending');
  const busy = inviteMutation.isPending || revokeMutation.isPending || resendMutation.isPending || removeMemberMutation.isPending;
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

  async function handleRemoveMember(member: TenantMemberRow): Promise<void> {
    const email = member.user?.email?.trim();
    const name = [member.user?.given_name, member.user?.family_name].map((part) => part?.trim()).filter(Boolean).join(' ');
    const fallbackName = member.user?.name?.trim();
    const label = email || name || (fallbackName && !isPlaceholderClerkName(fallbackName) ? fallbackName : '') || 'este usuario';
    if (!window.confirm(`¿Eliminar a ${label} del tenant?`)) {
      return;
    }
    try {
      setError('');
      setSuccess('');
      await removeMemberMutation.mutateAsync(member.user_id);
    } catch (err) {
      setError(formatFetchErrorForUser(err, 'No se pudo eliminar el usuario.'));
    }
  }

  return (
    <section className="card admin-settings-section">
      {error ? <p className="form-error">{error}</p> : null}
      {success ? <p className="form-success">{success}</p> : null}
      {!error && loadError ? (
        <p className="form-error">{formatFetchErrorForUser(loadError, 'No se pudo cargar el equipo.')}</p>
      ) : null}

      {canManageInvites ? (
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
                <th>Nombre</th>
                <th>Apellido</th>
                <th>Email</th>
                <th>Rol</th>
                <th>Estado</th>
                {canManageInvites ? <th>Acciones</th> : null}
              </tr>
            </thead>
            <tbody>
              {members.map((member) => {
                const name = memberNameParts(member);
                const canRemoveMember = canManageInvites && member.role !== 'owner';
                return (
                  <tr key={member.id}>
                    <td>{name.firstName}</td>
                    <td>{name.lastName}</td>
                    <td>{name.email}</td>
                    <td>
                      <code className="admin-code">{roleLabel(member.role)}</code>
                    </td>
                    <td>
                      <code className="admin-code">{member.status ?? 'active'}</code>
                    </td>
                    {canManageInvites ? (
                      <td>
                        {canRemoveMember ? (
                          <button
                            type="button"
                            className="btn-sm btn-danger"
                            disabled={busy}
                            onClick={() => void handleRemoveMember(member)}
                          >
                            Eliminar
                          </button>
                        ) : (
                          <span className="text-secondary">—</span>
                        )}
                      </td>
                    ) : null}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {canManageInvites ? (
        <div className="admin-settings-section admin-rbac-block-mt">
          <h3>Invitaciones</h3>
          {invitesQuery.isLoading ? <p className="text-secondary">Cargando invitaciones…</p> : null}
          {!invitesQuery.isLoading && invites.length === 0 ? (
            <p className="text-secondary">No hay invitaciones pendientes.</p>
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
