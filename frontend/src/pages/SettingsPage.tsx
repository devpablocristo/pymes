import { UserProfile } from '@clerk/clerk-react';
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { getMe, getSession } from '../lib/api';
import { clerkEnabled } from '../lib/auth';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { useI18n } from '../lib/i18n';
import type { MeProfileResponse, SessionResponse } from '../lib/types';

function authMethodLabel(t: (key: string) => string, method: string): string {
  const m = method.toLowerCase();
  if (m === 'jwt') return t('profile.authMethod.jwt');
  if (m === 'api_key') return t('profile.authMethod.api_key');
  if (m === '') return '—';
  return `${t('profile.authMethod.other')} (${method})`;
}

function ProfileSessionRows({
  session,
  t,
}: {
  session: SessionResponse;
  t: (key: string) => string;
}) {
  const { auth } = session;
  const productLabel = auth.product_role === 'admin' ? t('shell.role.admin') : t('shell.role.user');
  return (
    <table className="profile-session-table">
      <tbody>
        <tr>
          <th scope="row">{t('profile.labels.org')}</th>
          <td>
            <code className="mono">{auth.org_id || '—'}</code>
          </td>
        </tr>
        <tr>
          <th scope="row">{t('profile.labels.productRole')}</th>
          <td>
            <span className="badge badge-neutral">{productLabel}</span>
          </td>
        </tr>
        <tr>
          <th scope="row">{t('profile.labels.roleRaw')}</th>
          <td>
            <code className="mono">{auth.role || '—'}</code>
          </td>
        </tr>
        <tr>
          <th scope="row">{t('profile.labels.actor')}</th>
          <td>
            <code className="mono">{auth.actor || '—'}</code>
          </td>
        </tr>
        <tr>
          <th scope="row">{t('profile.labels.authMethod')}</th>
          <td>{authMethodLabel(t, auth.auth_method)}</td>
        </tr>
        <tr>
          <th scope="row">{t('profile.labels.scopes')}</th>
          <td>
            {auth.scopes.length === 0 ? (
              <span className="text-muted">—</span>
            ) : (
              auth.scopes.map((s) => (
                <span key={s} className="badge badge-neutral profile-scope-badge">
                  {s}
                </span>
              ))
            )}
          </td>
        </tr>
      </tbody>
    </table>
  );
}

function ApiKeyModeProfile() {
  const { t } = useI18n();
  const [loading, setLoading] = useState(true);
  const [session, setSession] = useState<SessionResponse | null>(null);
  const [me, setMe] = useState<MeProfileResponse | null>(null);
  const [error, setError] = useState('');
  const [meWarning, setMeWarning] = useState('');
  const [reloadToken, setReloadToken] = useState(0);

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

  const user = me?.user;

  return (
    <>
      <div className="page-header">
        <h1>Perfil</h1>
        <p>Gestiona tu cuenta y preferencias</p>
      </div>

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

      <div className="card">
        <div className="card-header">
          <h2>{t('profile.apiMode.title')}</h2>
        </div>
        <p className="profile-api-mode-lead">{t('profile.apiMode.lead')}</p>
        <p>
          <Link to="/settings/keys" className="btn-secondary">
            {t('profile.apiMode.keysCta')}
          </Link>
        </p>

        {loading ? (
          <p className="text-muted">{t('common.status.loading')}</p>
        ) : (
          <>
            <h3 className="profile-subsection-title">{t('profile.section.account')}</h3>
            {user ? (
              <table className="profile-session-table">
                <tbody>
                  <tr>
                    <th scope="row">{t('profile.labels.name')}</th>
                    <td>{user.name || '—'}</td>
                  </tr>
                  <tr>
                    <th scope="row">{t('profile.labels.email')}</th>
                    <td>{user.email || '—'}</td>
                  </tr>
                </tbody>
              </table>
            ) : (
              <p className="text-muted">{t('profile.accountPlaceholder')}</p>
            )}

            {session && (
              <>
                <h3 className="profile-subsection-title">{t('profile.section.session')}</h3>
                <ProfileSessionRows session={session} t={t} />
              </>
            )}
          </>
        )}
      </div>
    </>
  );
}

export function SettingsPage() {
  return clerkEnabled ? (
    <>
      <div className="page-header">
        <h1>Perfil</h1>
        <p>Gestiona tu cuenta y preferencias</p>
      </div>
      <div className="card">
        <UserProfile routing="path" path="/settings" />
      </div>
    </>
  ) : (
    <ApiKeyModeProfile />
  );
}
