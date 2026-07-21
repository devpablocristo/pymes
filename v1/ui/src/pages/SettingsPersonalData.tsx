import { useUser } from '@clerk/react';
import { useEffect, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { patchMeProfile } from '../lib/api';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { useI18n } from '../lib/i18n';
import { displayFamilyFromUser, displayGivenFromUser, mergeClerkSessionWithApiUser } from '../lib/profileDisplay';
import { queryKeys } from '../lib/queryKeys';
import type { MeProfileUser, SessionResponse } from '../lib/types';

export function PersonalDataForm({ displayUser, canEdit }: { displayUser: MeProfileUser | null; canEdit: boolean }) {
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

export function ClerkPersonalDataSection({
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
