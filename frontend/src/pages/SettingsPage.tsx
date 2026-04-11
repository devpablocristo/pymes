import { useAuth, useClerk, useOrganization, useOrganizationList, useSession, useUser } from '@clerk/react';
import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { AccountPlanSection } from '../components/AccountPlanSection';
import { PageLayout } from '../components/PageLayout';
import { getMe, getSession, getTenantSettings, patchMeProfile, updateTenantSettings } from '../lib/api';
import { clerkEnabled } from '../lib/auth';
import { clearTenantProfile, syncTenantProfileFromSettings } from '../lib/tenantProfile';
import { formatClerkAPIUserMessage } from '../lib/clerkErrors';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { useI18n } from '../lib/i18n';
import { displayFamilyFromUser, displayGivenFromUser, mergeClerkSessionWithApiUser } from '../lib/profileDisplay';
import { queryKeys } from '../lib/queryKeys';
import type { MeProfileResponse, MeProfileUser, SessionResponse } from '../lib/types';

/** Evita spinner infinito si Clerk/getToken o la red no resuelven. */
const PROFILE_LOAD_TIMEOUT_MS = 45_000;

function rejectAfterMs(ms: number, message: string): Promise<never> {
  return new Promise((_, reject) => {
    window.setTimeout(() => reject(new Error(message)), ms);
  });
}

async function getSessionWithTimeout(): Promise<SessionResponse> {
  return Promise.race([getSession(), rejectAfterMs(PROFILE_LOAD_TIMEOUT_MS, 'profile_fetch_timeout')]);
}

async function getMeWithTimeout(): Promise<MeProfileResponse> {
  return Promise.race([getMe(), rejectAfterMs(PROFILE_LOAD_TIMEOUT_MS, 'profile_fetch_timeout')]);
}

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
  hideOrgRow = false,
}: {
  session: SessionResponse;
  /** Nombre de la org activa en Clerk (solo modo Clerk); prioridad sobre org_name del API. */
  clerkOrgName?: string | null;
  t: (key: string) => string;
  /** En modo Clerk la org se edita en un bloque aparte debajo de esta tabla. */
  hideOrgRow?: boolean;
}) {
  const { auth } = session;
  const orgLabel = profileOrgLabel(auth, clerkOrgName);
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

function ProfileAccountBlock({ user }: { user: MeProfileUser }) {
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

/** Nombre de organización: solo aquí (y en onboarding), sin OrganizationSwitcher en la barra. */
function ClerkOrganizationNameSection({ t }: { t: (key: string) => string }) {
  const { organization, isLoaded: orgLoaded } = useOrganization();
  const { orgRole, isLoaded: authLoaded } = useAuth();

  const [editing, setEditing] = useState(false);
  const [nameEdit, setNameEdit] = useState('');
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState('');
  const [savedHint, setSavedHint] = useState(false);

  useEffect(() => {
    if (!organization || editing) {
      return;
    }
    setNameEdit(organization.name.trim());
    // eslint-disable-next-line react-hooks/exhaustive-deps -- sincronizar solo cuando cambia id/name, no el objeto entero
  }, [editing, organization?.id, organization?.name]);

  function handleCancelEdit(): void {
    if (organization) {
      setNameEdit(organization.name.trim());
    }
    setEditing(false);
    setFormError('');
    setSavedHint(false);
  }

  async function handleSave(): Promise<void> {
    if (!organization) {
      return;
    }
    const nextName = nameEdit.trim();
    if (nextName.length < 2) {
      setFormError(t('profile.org.validationMin'));
      return;
    }
    setFormError('');
    setSavedHint(false);
    setSaving(true);
    try {
      await organization.update({ name: nextName });
      setSavedHint(true);
      setEditing(false);
    } catch (err) {
      setFormError(formatClerkAPIUserMessage(err, t('profile.org.saveError')));
    } finally {
      setSaving(false);
    }
  }

  if (!orgLoaded || !authLoaded) {
    return <p className="text-muted">{t('common.status.loading')}</p>;
  }

  if (!organization) {
    return <p className="text-muted">{t('profile.org.noOrganization')}</p>;
  }

  const canEdit = orgRole === 'org:admin';

  if (!canEdit) {
    return (
      <div className="profile-org-readonly">
        <dl className="profile-readonly-dl">
          <div>
            <dt>{t('profile.labels.org')}</dt>
            <dd>
              <span className="profile-session-value">{organization.name.trim() || '—'}</span>
            </dd>
          </div>
        </dl>
        <p className="text-muted profile-org-member-hint">{t('profile.org.readOnlyMember')}</p>
      </div>
    );
  }

  if (!editing) {
    return (
      <div className="profile-personal-form profile-personal-form--readonly profile-org-name-block">
        <dl className="profile-readonly-dl">
          <div>
            <dt>{t('profile.labels.org')}</dt>
            <dd>
              <span className="profile-session-value">{organization.name.trim() || '—'}</span>
            </dd>
          </div>
        </dl>
        <p className="text-muted profile-field-hint">{t('profile.org.nameHint')}</p>
        <p className="profile-form-actions">
          <button type="button" className="btn-secondary" onClick={() => setEditing(true)}>
            {t('profile.org.edit')}
          </button>
        </p>
        {savedHint && <p className="text-muted profile-saved-hint">{t('profile.org.saved')}</p>}
      </div>
    );
  }

  return (
    <div className="profile-personal-form profile-org-name-block">
      <label className="profile-field-label" htmlFor="profile-org-name">
        {t('profile.labels.org')}
      </label>
      <input
        id="profile-org-name"
        className="input profile-input"
        value={nameEdit}
        onChange={(e) => setNameEdit(e.target.value)}
        autoComplete="organization"
        maxLength={100}
      />
      {formError && <p className="alert alert-error profile-form-alert">{formError}</p>}
      <p className="profile-form-actions profile-form-actions--edit">
        <button type="button" className="btn-primary" disabled={saving} onClick={() => void handleSave()}>
          {saving ? t('profile.org.saving') : t('profile.org.save')}
        </button>
        <button type="button" className="btn-secondary" disabled={saving} onClick={handleCancelEdit}>
          {t('profile.personal.cancel')}
        </button>
      </p>
    </div>
  );
}

function ClerkOrganizationSwitcherSection({ t }: { t: (key: string) => string }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { orgId, isLoaded: authLoaded } = useAuth();
  const { session } = useSession();
  const clerk = useClerk();
  const {
    isLoaded: listLoaded,
    userMemberships,
  } = useOrganizationList({
    userMemberships: { pageSize: 50 },
  });

  const [newOrgName, setNewOrgName] = useState('');
  const [switchingOrgID, setSwitchingOrgID] = useState<string | null>(null);
  const [switchError, setSwitchError] = useState('');
  const [reopeningOnboarding, setReopeningOnboarding] = useState(false);

  const memberships = userMemberships.data ?? [];

  async function activateOrganization(targetOrgID: string): Promise<void> {
    setSwitchError('');
    setSwitchingOrgID(targetOrgID);
    try {
      clearTenantProfile();
      await clerk.setActive({ organization: targetOrgID });
      await session?.reload();
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.session.current }),
        queryClient.invalidateQueries({ queryKey: queryKeys.me.current }),
        queryClient.invalidateQueries({ queryKey: queryKeys.tenant.settings }),
      ]);
      const tenantSettings = await queryClient.fetchQuery({
        queryKey: queryKeys.tenant.settings,
        queryFn: getTenantSettings,
      });
      syncTenantProfileFromSettings(tenantSettings);
      navigate(tenantSettings.onboarding_completed_at ? '/' : '/onboarding', { replace: true });
    } catch (err) {
      setSwitchError(formatClerkAPIUserMessage(err, t('profile.org.switchError')));
    } finally {
      setSwitchingOrgID(null);
    }
  }

  async function handleCreateOrganization(): Promise<void> {
    const name = newOrgName.trim();
    if (name.length < 2) {
      setSwitchError(t('profile.org.validationMin'));
      return;
    }
    setSwitchError('');
    setSwitchingOrgID('__new__');
    try {
      const created = await clerk.createOrganization({ name });
      setNewOrgName('');
      await activateOrganization(created.id);
    } catch (err) {
      setSwitchError(formatClerkAPIUserMessage(err, t('profile.org.switchError')));
      setSwitchingOrgID(null);
    }
  }

  async function handleReopenOnboarding(): Promise<void> {
    setSwitchError('');
    setReopeningOnboarding(true);
    try {
      clearTenantProfile();
      const tenantSettings = await updateTenantSettings({ onboarding_completed_at: null });
      queryClient.setQueryData(queryKeys.tenant.settings, tenantSettings);
      syncTenantProfileFromSettings(tenantSettings);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.session.current }),
        queryClient.invalidateQueries({ queryKey: queryKeys.me.current }),
        queryClient.invalidateQueries({ queryKey: queryKeys.tenant.settings }),
      ]);
      navigate('/onboarding', { replace: true });
    } catch (err) {
      setSwitchError(formatFetchErrorForUser(err, t('profile.org.switchError')));
    } finally {
      setReopeningOnboarding(false);
    }
  }

  if (!authLoaded || !listLoaded) {
    return <p className="text-muted">{t('common.status.loading')}</p>;
  }

  return (
    <div className="profile-org-switcher">
      <p className="text-muted profile-field-hint">{t('profile.org.switchLead')}</p>
      <div className="profile-org-switcher__list">
        {memberships.map((membership) => {
          const membershipOrgID = membership.organization?.id ?? '';
          const isCurrent = membershipOrgID !== '' && membershipOrgID === orgId;
          const isBusy = switchingOrgID === membershipOrgID;
          return (
            <div key={membershipOrgID || membership.id} className="profile-org-switcher__item">
              <div>
                <strong>{membership.organization?.name?.trim() || membershipOrgID || '—'}</strong>
                {isCurrent ? (
                  <div className="text-muted">{t('profile.org.switchCurrent')}</div>
                ) : null}
              </div>
              <button
                type="button"
                className="btn-secondary"
                disabled={isCurrent || isBusy || switchingOrgID != null || !membershipOrgID}
                onClick={() => void activateOrganization(membershipOrgID)}
              >
                {isBusy ? t('profile.org.switchLoading') : isCurrent ? t('profile.org.switchCurrent') : t('profile.org.switchUse')}
              </button>
            </div>
          );
        })}
        {memberships.length === 0 ? <p className="text-muted">{t('profile.org.switchEmpty')}</p> : null}
      </div>
      <div className="profile-org-switcher__create">
        <label className="profile-field-label" htmlFor="profile-org-create">
          {t('profile.org.switchCreateLabel')}
        </label>
        <input
          id="profile-org-create"
          className="input profile-input"
          value={newOrgName}
          onChange={(e) => setNewOrgName(e.target.value)}
          placeholder={t('profile.org.switchCreatePlaceholder')}
          maxLength={100}
        />
        <p className="profile-form-actions">
          <button
            type="button"
            className="btn-primary"
            disabled={switchingOrgID != null}
            onClick={() => void handleCreateOrganization()}
          >
            {switchingOrgID === '__new__' ? t('profile.org.switchLoading') : t('profile.org.switchCreateAction')}
          </button>
        </p>
      </div>
      <div className="profile-org-switcher__reset">
        <p className="text-muted profile-field-hint">{t('profile.org.reopenOnboardingHint')}</p>
        <p className="profile-form-actions">
          <button
            type="button"
            className="btn-secondary"
            disabled={switchingOrgID != null || reopeningOnboarding}
            onClick={() => void handleReopenOnboarding()}
          >
            {reopeningOnboarding ? t('profile.org.reopenOnboardingLoading') : t('profile.org.reopenOnboarding')}
          </button>
        </p>
      </div>
      {switchError ? <p className="alert alert-error profile-form-alert">{switchError}</p> : null}
    </div>
  );
}

/** Solo se monta en modo Clerk para poder usar useOrganization sin romper el build sin ClerkProvider. */
function ClerkProfileSessionRows({
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
function ClerkAccountSignOutButton() {
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

function LocalAccountSignOutButton() {
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

function PersonalDataForm({ displayUser, canEdit }: { displayUser: MeProfileUser | null; canEdit: boolean }) {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [givenEdit, setGivenEdit] = useState('');
  const [familyEdit, setFamilyEdit] = useState('');
  const [phoneEdit, setPhoneEdit] = useState('');
  const [formError, setFormError] = useState('');
  const [savedHint, setSavedHint] = useState(false);
  const saveProfileMutation = useMutation({
    mutationFn: async () =>
      patchMeProfile({
        given_name: givenEdit.trim(),
        family_name: familyEdit.trim(),
        phone: phoneEdit.trim(),
      }),
    onSuccess: (next) => {
      queryClient.setQueryData(queryKeys.me.current, next);
      setSavedHint(true);
      setEditing(false);
    },
    onError: (e) => {
      setFormError(formatFetchErrorForUser(e, t('profile.error.unreachable')));
    },
  });

  function syncFieldsFromDisplay(): void {
    if (!displayUser) {
      return;
    }
    setGivenEdit(displayGivenFromUser(displayUser));
    setFamilyEdit(displayFamilyFromUser(displayUser));
    setPhoneEdit((displayUser.phone ?? '').trim());
  }

  useEffect(() => {
    if (!displayUser || editing) {
      return;
    }
    syncFieldsFromDisplay();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- sincronizar solo cuando cambian los datos del user, no el objeto/función
  }, [
    editing,
    displayUser?.id,
    displayUser?.name,
    displayUser?.given_name,
    displayUser?.family_name,
    displayUser?.phone,
  ]);

  function handleCancelEdit(): void {
    syncFieldsFromDisplay();
    setEditing(false);
    setFormError('');
    setSavedHint(false);
  }

  async function handleSave(): Promise<void> {
    if (!canEdit || !displayUser) {
      return;
    }
    setFormError('');
    setSavedHint(false);
    try {
      await saveProfileMutation.mutateAsync();
    } catch {
      // El mensaje ya se resuelve en `onError`.
    }
  }

  if (!canEdit) {
    return (
      <>
        <p className="text-muted profile-personal-readonly-hint">{t('profile.personal.readOnlyJwt')}</p>
        {displayUser && (
          <dl className="profile-readonly-dl">
            <div className="profile-readonly-name-row">
              <div>
                <dt>{t('profile.labels.givenName')}</dt>
                <dd>{displayGivenFromUser(displayUser) || '—'}</dd>
              </div>
              <div>
                <dt>{t('profile.labels.familyName')}</dt>
                <dd>{displayFamilyFromUser(displayUser) || '—'}</dd>
              </div>
            </div>
            <div>
              <dt>{t('profile.labels.phone')}</dt>
              <dd>{(displayUser.phone ?? '').trim() || '—'}</dd>
            </div>
          </dl>
        )}
      </>
    );
  }

  if (!displayUser) {
    return <p className="text-muted">{t('profile.personal.noUser')}</p>;
  }

  if (!editing) {
    return (
      <div className="profile-personal-form profile-personal-form--readonly">
        <dl className="profile-readonly-dl">
          <div className="profile-readonly-name-row">
            <div>
              <dt>{t('profile.labels.givenName')}</dt>
              <dd>{displayGivenFromUser(displayUser) || '—'}</dd>
            </div>
            <div>
              <dt>{t('profile.labels.familyName')}</dt>
              <dd>{displayFamilyFromUser(displayUser) || '—'}</dd>
            </div>
          </div>
          <div>
            <dt>{t('profile.labels.phone')}</dt>
            <dd>{(displayUser.phone ?? '').trim() || '—'}</dd>
          </div>
        </dl>
        <p className="profile-form-actions">
          <button type="button" className="btn-secondary" onClick={() => setEditing(true)}>
            {t('profile.personal.edit')}
          </button>
        </p>
        {savedHint && <p className="text-muted profile-saved-hint">{t('profile.personal.saved')}</p>}
      </div>
    );
  }

  return (
    <div className="profile-personal-form">
      <div className="profile-name-row">
        <div className="profile-name-field">
          <label className="profile-field-label" htmlFor="profile-given">
            {t('profile.labels.givenName')}
          </label>
          <input
            id="profile-given"
            className="input profile-input"
            value={givenEdit}
            onChange={(e) => setGivenEdit(e.target.value)}
            autoComplete="given-name"
            maxLength={100}
          />
        </div>
        <div className="profile-name-field">
          <label className="profile-field-label" htmlFor="profile-family">
            {t('profile.labels.familyName')}
          </label>
          <input
            id="profile-family"
            className="input profile-input"
            value={familyEdit}
            onChange={(e) => setFamilyEdit(e.target.value)}
            autoComplete="family-name"
            maxLength={100}
          />
        </div>
      </div>
      <p className="text-muted profile-field-hint">{t('profile.labels.phoneHint')}</p>
      <label className="profile-field-label" htmlFor="profile-phone">
        {t('profile.labels.phone')}
      </label>
      <input
        id="profile-phone"
        className="input profile-input"
        type="tel"
        value={phoneEdit}
        onChange={(e) => setPhoneEdit(e.target.value)}
        autoComplete="tel"
        maxLength={40}
      />
      {formError && <p className="alert alert-error profile-form-alert">{formError}</p>}
      <p className="profile-form-actions profile-form-actions--edit">
        <button
          type="button"
          className="btn-primary"
          disabled={saveProfileMutation.isPending}
          onClick={() => void handleSave()}
        >
          {saveProfileMutation.isPending ? t('profile.personal.saving') : t('profile.personal.save')}
        </button>
        <button
          type="button"
          className="btn-secondary"
          disabled={saveProfileMutation.isPending}
          onClick={handleCancelEdit}
        >
          {t('profile.personal.cancel')}
        </button>
      </p>
    </div>
  );
}

function ClerkPersonalDataSection({
  apiUser,
  session,
}: {
  apiUser: MeProfileUser | null | undefined;
  session: SessionResponse;
}) {
  const { t } = useI18n();
  const { isLoaded, user: clerkUser } = useUser();
  const canEdit = session.auth.auth_method === 'jwt';

  if (!isLoaded) {
    return <p className="text-muted">{t('common.status.loading')}</p>;
  }

  const displayUser = clerkUser ? mergeClerkSessionWithApiUser(clerkUser, apiUser ?? undefined) : (apiUser ?? null);

  return <PersonalDataForm displayUser={displayUser} canEdit={canEdit} />;
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

function SettingsProfileBody({ clerkMode }: { clerkMode: boolean }) {
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

type SettingsPageProps = {
  embedded?: boolean;
};

export function SettingsPage({ embedded = false }: SettingsPageProps = {}) {
  const { t } = useI18n();
  const body = <SettingsProfileBody clerkMode={clerkEnabled} />;

  if (embedded) {
    return <>{body}</>;
  }

  return (
    <PageLayout className="profile-page" title={t('profile.page.title')} lead={t('profile.page.subtitle')}>
      {body}
    </PageLayout>
  );
}

/** Sección de facturación standalone para usar en tabs de ajustes. */
export function BillingSettingsSection() {
  const sessionQuery = useQuery({
    queryKey: queryKeys.session.current,
    queryFn: getSessionWithTimeout,
    retry: false,
  });
  if (!sessionQuery.data) return <div className="spinner" />;
  return <AccountPlanSection session={sessionQuery.data} />;
}
