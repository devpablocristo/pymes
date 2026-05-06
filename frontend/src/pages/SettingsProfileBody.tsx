import { useQuery } from '@tanstack/react-query';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { useI18n } from '../lib/i18n';
import { queryKeys } from '../lib/queryKeys';
import { getMeWithTimeout, getSessionWithTimeout } from './SettingsPage.data';
import { ClerkOrganizationNameSection, ClerkOrganizationSwitcherSection } from './SettingsOrganizationSections';
import { ClerkPersonalDataSection, PersonalDataForm } from './SettingsPersonalData';
import {
  ClerkAccountSignOutButton,
  ClerkProfileAccountSection,
  ClerkProfileSessionRows,
  LocalAccountSignOutButton,
  ProfileAccountBlock,
  ProfileSessionRows,
} from './SettingsProfileAccount';

export function SettingsProfileBody({ clerkMode }: { clerkMode: boolean }) {
  const { t } = useI18n();
  const sessionQuery = useQuery({
    queryKey: queryKeys.session.current,
    queryFn: getSessionWithTimeout,
    retry: false,
  });
  const meQuery = useQuery({
    queryKey: queryKeys.me.current,
    queryFn: getMeWithTimeout,
    retry: false,
  });

  const session = sessionQuery.data ?? null;
  const me = meQuery.data ?? null;
  const user = me?.user;
  const loading = sessionQuery.isLoading || meQuery.isLoading;
  const error = sessionQuery.error ? formatFetchErrorForUser(sessionQuery.error, t('profile.error.unreachable')) : '';
  const meWarning =
    !sessionQuery.error && meQuery.error
      ? formatFetchErrorForUser(meQuery.error, t('profile.error.meUnreachable'))
      : '';
  const accountLoadFailed = Boolean(meWarning);
  const refetchProfile = () => {
    void sessionQuery.refetch();
    void meQuery.refetch();
  };

  return (
    <>
      {error && (
        <div className="alert alert-error">
          <p>{error}</p>
          <p>
            <button type="button" className="btn-secondary btn-sm" onClick={refetchProfile}>
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
        </div>
      )}

      {loading ? (
        <div className="card">
          <p className="text-muted">{t('common.status.loading')}</p>
        </div>
      ) : (
        <>
          <div className="card profile-section-card">
            <div className="card-header">
              <h2>{t('profile.section.account')}</h2>
            </div>
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
            {session &&
              (clerkMode ? (
                <ClerkProfileSessionRows session={session} t={t} hideOrgRow />
              ) : (
                <ProfileSessionRows session={session} clerkOrgName={null} t={t} />
              ))}
            {clerkMode ? <ClerkAccountSignOutButton /> : <LocalAccountSignOutButton />}
            {clerkMode && (
              <div className="profile-org-after-signout">
                <ClerkOrganizationNameSection t={t} />
                <div className="profile-org-switcher-wrap">
                  <h3 className="profile-subsection-title">{t('profile.org.switchTitle')}</h3>
                  <ClerkOrganizationSwitcherSection t={t} />
                </div>
              </div>
            )}
          </div>

          {session && (
            <div className="card profile-section-card">
              <div className="card-header">
                <h2>{t('profile.section.personal')}</h2>
              </div>
              {clerkMode ? (
                <ClerkPersonalDataSection apiUser={user ?? undefined} session={session} />
              ) : (
                <PersonalDataForm displayUser={user ?? null} canEdit={session.auth.auth_method === 'jwt'} />
              )}
            </div>
          )}
        </>
      )}
    </>
  );
}
