import { useAuth, useClerk, useOrganization, useOrganizationList, useSession } from '@clerk/react';
import { useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { getTenantSettings, updateTenantSettings } from '../lib/api';
import { formatClerkAPIUserMessage } from '../lib/clerkErrors';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { queryKeys } from '../lib/queryKeys';
import { clearTenantProfile, syncTenantProfileFromSettings } from '../lib/tenantProfile';

/** Nombre de tenant: solo aquí (y en onboarding), sin OrganizationSwitcher en la barra. */
export function ClerkOrganizationNameSection({ t }: { t: (key: string) => string }) {
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

export function ClerkOrganizationSwitcherSection({ t }: { t: (key: string) => string }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { orgId, isLoaded: authLoaded } = useAuth();
  const { session } = useSession();
  const clerk = useClerk();
  const { isLoaded: listLoaded, userMemberships } = useOrganizationList({
    userMemberships: { pageSize: 50 },
  });

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
        queryFn: () => getTenantSettings(),
      });
      syncTenantProfileFromSettings(tenantSettings);
      navigate(tenantSettings.onboarding_completed_at ? '/' : '/onboarding', { replace: true });
    } catch (err) {
      setSwitchError(formatClerkAPIUserMessage(err, t('profile.org.switchError')));
    } finally {
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
                {isCurrent ? <div className="text-muted">{t('profile.org.switchCurrent')}</div> : null}
              </div>
              <button
                type="button"
                className="btn-secondary"
                disabled={isCurrent || isBusy || switchingOrgID != null || !membershipOrgID}
                onClick={() => void activateOrganization(membershipOrgID)}
              >
                {isBusy
                  ? t('profile.org.switchLoading')
                  : isCurrent
                    ? t('profile.org.switchCurrent')
                    : t('profile.org.switchUse')}
              </button>
            </div>
          );
        })}
        {memberships.length === 0 ? <p className="text-muted">{t('profile.org.switchEmpty')}</p> : null}
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
