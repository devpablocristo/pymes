import { useOrganization, useUser } from '@clerk/clerk-react';
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { getMe, getSession } from '../lib/api';
import { clerkEnabled } from '../lib/auth';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { useI18n } from '../lib/i18n';
import type { MeProfileResponse, MeProfileUser, SessionResponse } from '../lib/types';

function profileOrgLabel(auth: SessionResponse['auth'], clerkOrgName: string | null | undefined): string {
  const clerk = clerkOrgName?.trim() || '';
  const apiName = typeof auth.org_name === 'string' ? auth.org_name.trim() : '';
  const id = auth.org_id?.trim() || '';
  return clerk || apiName || id || '—';
}

function accountTypeLabel(t: (key: string) => string, productRole: SessionResponse['auth']['product_role']): string {
  return productRole === 'admin' ? t('profile.accountTypeValue.admin') : t('profile.accountTypeValue.user');
}

function ProfileSessionRows({
  session,
  clerkOrgName,
  t,
}: {
  session: SessionResponse;
  /** Nombre de la org activa en Clerk (solo modo Clerk); prioridad sobre org_name del API. */
  clerkOrgName?: string | null;
  t: (key: string) => string;
}) {
  const { auth } = session;
  const orgLabel = profileOrgLabel(auth, clerkOrgName);
  const typeLabel = accountTypeLabel(t, auth.product_role);
  return (
    <table className="profile-session-table">
      <tbody>
        <tr>
          <th scope="row">{t('profile.labels.org')}</th>
          <td>
            <span className="profile-session-value">{orgLabel}</span>
          </td>
        </tr>
        <tr>
          <th scope="row">{t('profile.labels.accountType')}</th>
          <td>
            <span className="profile-session-value">{typeLabel}</span>
          </td>
        </tr>
      </tbody>
    </table>
  );
}

function ProfileAccountBlock({ user }: { user: MeProfileUser }) {
  const initial =
    user.name?.trim().charAt(0)?.toUpperCase() ||
    user.email?.trim().charAt(0)?.toUpperCase() ||
    '?';

  return (
    <div className="profile-account-block">
      {user.avatar_url ? (
        <img className="profile-account-avatar" src={user.avatar_url} alt="" width={64} height={64} />
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

/** Enriquece la tarjeta Cuenta: el JWT suele no traer email/nombre; Clerk en el browser sí. */
function mergeClerkSessionWithApiUser(
  clerkUser: NonNullable<ReturnType<typeof useUser>['user']>,
  apiUser: MeProfileUser | null | undefined,
): MeProfileUser {
  const email =
    clerkUser.primaryEmailAddress?.emailAddress?.trim() ||
    apiUser?.email?.trim() ||
    '';
  const nameFromClerk =
    (typeof clerkUser.fullName === 'string' ? clerkUser.fullName.trim() : '') ||
    [clerkUser.firstName, clerkUser.lastName].filter(Boolean).join(' ').trim() ||
    clerkUser.username?.trim() ||
    '';
  const name = nameFromClerk || apiUser?.name?.trim() || '';
  return {
    id: apiUser?.id || clerkUser.id,
    external_id: apiUser?.external_id || clerkUser.id,
    email,
    name,
    avatar_url: clerkUser.imageUrl || apiUser?.avatar_url || null,
  };
}

/** Solo se monta en modo Clerk para poder usar useOrganization sin romper el build sin ClerkProvider. */
function ClerkProfileSessionRows({ session, t }: { session: SessionResponse; t: (key: string) => string }) {
  const { organization } = useOrganization();
  const clerkOrgName = organization?.name?.trim() || null;
  return <ProfileSessionRows session={session} clerkOrgName={clerkOrgName} t={t} />;
}

function ClerkProfileAccountSection({
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
    return <ProfileAccountBlock user={mergeClerkSessionWithApiUser(clerkUser, apiUser)} />;
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

function SettingsProfileBody({ clerkMode }: { clerkMode: boolean }) {
  const { t } = useI18n();
  const [loading, setLoading] = useState(true);
  const [session, setSession] = useState<SessionResponse | null>(null);
  const [me, setMe] = useState<MeProfileResponse | null>(null);
  const [error, setError] = useState('');
  const [meWarning, setMeWarning] = useState('');
  const [reloadToken, setReloadToken] = useState(0);

  const user = me?.user;
  const accountLoadFailed = Boolean(meWarning);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const [sessionRes, meRes] = await Promise.allSettled([getSession(), getMe()]);
        if (cancelled) return;
        if (sessionRes.status === 'fulfilled') {
          setSession(sessionRes.value);
          setError('');
        } else {
          setSession(null);
          setError(formatFetchErrorForUser(sessionRes.reason, t('profile.error.unreachable')));
        }
        if (meRes.status === 'fulfilled') {
          setMe(meRes.value);
          setMeWarning('');
        } else {
          setMe(null);
          setMeWarning(formatFetchErrorForUser(meRes.reason, t('profile.error.meUnreachable')));
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [reloadToken, t]);

  return (
    <>
      {error && (
        <div className="alert alert-error">
          <p>{error}</p>
          <p>
            <button type="button" className="btn-secondary btn-sm" onClick={() => setReloadToken((n) => n + 1)}>
              {t('profile.actions.retry')}
            </button>
          </p>
        </div>
      )}

      {meWarning && !error && <div className="alert alert-warning">{meWarning}</div>}

      {!clerkMode && (
        <div className="card">
          <div className="card-header profile-card-header-api">
            <h2>{t('profile.apiMode.title')}</h2>
            <span className="badge badge-neutral profile-mode-badge">{t('profile.apiMode.badge')}</span>
          </div>
          <p className="profile-api-mode-lead">{t('profile.apiMode.lead')}</p>
          <p>
            <Link to="/settings/keys" className="btn-secondary">
              {t('profile.apiMode.keysCta')}
            </Link>
          </p>
        </div>
      )}

      <div className="card">
        {loading ? (
          <p className="text-muted">{t('common.status.loading')}</p>
        ) : (
          <>
            <h3 className="profile-subsection-title profile-subsection-title--first">{t('profile.section.account')}</h3>
            {clerkMode ? (
              <ClerkProfileAccountSection apiUser={user ?? undefined} accountLoadFailed={accountLoadFailed} />
            ) : user ? (
              <ProfileAccountBlock user={user} />
            ) : accountLoadFailed ? (
              <p className="text-muted">{t('profile.account.unavailable')}</p>
            ) : (
              <div className="profile-account-panel" role="status">
                <p className="profile-account-panel-title">{t('profile.account.empty.title')}</p>
                <p className="profile-account-panel-body">{t('profile.account.empty.body')}</p>
              </div>
            )}

            {session && (
              <>
                <hr className="profile-section-divider" aria-hidden="true" />
                <h3 className="profile-subsection-title profile-subsection-title--after-divider">
                  {t('profile.section.session')}
                </h3>
                {clerkMode ? (
                  <ClerkProfileSessionRows session={session} t={t} />
                ) : (
                  <ProfileSessionRows session={session} clerkOrgName={null} t={t} />
                )}
              </>
            )}
          </>
        )}
      </div>
    </>
  );
}

export function SettingsPage() {
  const { t } = useI18n();
  return (
    <div className="profile-page">
      <div className="page-header">
        <h1>{t('profile.page.title')}</h1>
        <p>{t('profile.page.subtitle')}</p>
      </div>
      <SettingsProfileBody clerkMode={clerkEnabled} />
    </div>
  );
}
