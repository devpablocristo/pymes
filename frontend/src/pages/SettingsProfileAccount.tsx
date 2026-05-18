import { useClerk, useOrganization, useUser } from '@clerk/react';
import { useNavigate } from 'react-router-dom';
import { clearTenantProfile } from '../lib/tenantProfile';
import { useI18n } from '../lib/i18n';
import { mergeClerkSessionWithApiUser } from '../lib/profileDisplay';
import type { MeProfileUser, SessionResponse } from '../lib/types';
import { accountTypeLabel, profileTenantLabel } from './SettingsPage.data';

export function ProfileSessionRows({
  session,
  clerkOrgName,
  t,
  hideOrgRow = false,
}: {
  session: SessionResponse;
  /** Nombre del tenant activo en Clerk; prioridad sobre tenant_name del API. */
  clerkOrgName?: string | null;
  t: (key: string) => string;
  /** En modo Clerk la org se edita en un bloque aparte debajo de esta tabla. */
  hideOrgRow?: boolean;
}) {
  const { auth } = session;
  const orgLabel = profileTenantLabel(auth, clerkOrgName);
  const typeLabel = accountTypeLabel(t, auth.product_role);
  return (
    <table className="profile-session-table">
      <tbody>
        <tr>
          <th scope="row">{t('profile.labels.accountType')}</th>
          <td>
            <span className="profile-session-value">{typeLabel}</span>
          </td>
        </tr>
        {!hideOrgRow && (
          <tr>
            <th scope="row">{t('profile.labels.org')}</th>
            <td>
              <span className="profile-session-value">{orgLabel}</span>
            </td>
          </tr>
        )}
      </tbody>
    </table>
  );
}

export function ProfileAccountBlock({ user }: { user: MeProfileUser }) {
  const initial = user.name?.trim().charAt(0)?.toUpperCase() || user.email?.trim().charAt(0)?.toUpperCase() || '?';

  return (
    <div className="profile-account-block">
      {user.avatar_url ? (
        <img
          className="profile-account-avatar"
          src={user.avatar_url}
          alt={`Avatar de ${user.name ?? user.email ?? 'usuario'}`}
          width={64}
          height={64}
        />
      ) : (
        <div className="profile-account-avatar profile-account-avatar-placeholder" aria-hidden>
          {initial}
        </div>
      )}
      <div className="profile-account-text">
        <p className="profile-account-name">{user.name?.trim() || '—'}</p>
        <p className="profile-account-email text-muted">{user.email?.trim() || '—'}</p>
      </div>
    </div>
  );
}

/** Solo se monta en modo Clerk para poder usar useOrganization sin romper el build sin ClerkProvider. */
export function ClerkProfileSessionRows({
  session,
  t,
  hideOrgRow,
}: {
  session: SessionResponse;
  t: (key: string) => string;
  hideOrgRow?: boolean;
}) {
  const { organization } = useOrganization();
  const clerkOrgName = organization?.name?.trim() || null;
  return <ProfileSessionRows session={session} clerkOrgName={clerkOrgName} t={t} hideOrgRow={hideOrgRow} />;
}

/** Solo con ClerkProvider montado (perfil en modo Clerk). */
export function ClerkAccountSignOutButton() {
  const { signOut } = useClerk();
  const { t } = useI18n();

  async function handleSignOut(): Promise<void> {
    await signOut({ redirectUrl: '/login' });
  }

  return (
    <p className="profile-account-signout">
      <button type="button" className="btn-secondary" onClick={() => void handleSignOut()}>
        {t('profile.account.signOut')}
      </button>
    </p>
  );
}

export function LocalAccountSignOutButton() {
  const navigate = useNavigate();
  const { t } = useI18n();

  function handleLeave(): void {
    clearTenantProfile();
    navigate('/onboarding', { replace: true });
  }

  return (
    <p className="profile-account-signout">
      <button type="button" className="btn-secondary" onClick={handleLeave}>
        {t('profile.account.signOut')}
      </button>
    </p>
  );
}

export function ClerkProfileAccountSection({
  apiUser,
  accountLoadFailed,
}: {
  apiUser: MeProfileUser | null | undefined;
  accountLoadFailed: boolean;
}) {
  const { t } = useI18n();
  const { isLoaded, user: clerkUser } = useUser();

  if (!isLoaded) {
    return <p className="text-muted">{t('common.status.loading')}</p>;
  }

  if (clerkUser) {
    const merged = mergeClerkSessionWithApiUser(clerkUser, apiUser);
    return <ProfileAccountBlock user={merged} />;
  }

  if (apiUser) {
    return <ProfileAccountBlock user={apiUser} />;
  }

  if (accountLoadFailed) {
    return <p className="text-muted">{t('profile.account.unavailable')}</p>;
  }

  return (
    <div className="profile-account-panel" role="status">
      <p className="profile-account-panel-title">{t('profile.account.empty.title')}</p>
      <p className="profile-account-panel-body">{t('profile.account.empty.clerk')}</p>
    </div>
  );
}
