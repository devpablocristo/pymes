import { FormEvent, useEffect, useState } from 'react';
import { createAPIKey, deleteAPIKey, getAPIKeys, getSession, rotateAPIKey } from '../lib/api';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { useI18n } from '../lib/i18n';
import type { APIKeyItem, SessionResponse } from '../lib/types';

/** Scopes que usa el core para acceso a consola/API con clave (ver authz en pymes-core). */
const SCOPE_CONSOLE_READ = 'admin:console:read';
const SCOPE_CONSOLE_WRITE = 'admin:console:write';

function scopeBadgeLabel(scope: string, t: (key: string) => string): string {
  if (scope === SCOPE_CONSOLE_READ) return t('apiKeys.scopes.consoleRead.label');
  if (scope === SCOPE_CONSOLE_WRITE) return t('apiKeys.scopes.consoleWrite.label');
  return scope;
}

export function APIKeysPage() {
  const { t } = useI18n();
  const [sessionLoading, setSessionLoading] = useState(true);
  const [session, setSession] = useState<SessionResponse | null>(null);
  const [orgID, setOrgID] = useState('');
  const [keys, setKeys] = useState<APIKeyItem[]>([]);
  const [name, setName] = useState('');
  const [scopeConsoleRead, setScopeConsoleRead] = useState(true);
  const [scopeConsoleWrite, setScopeConsoleWrite] = useState(true);
  const [newRawKey, setNewRawKey] = useState('');
  const [error, setError] = useState('');

  const canManage = session?.auth.product_role === 'admin';

  async function loadKeys(resolvedOrgID: string): Promise<void> {
    const list = await getAPIKeys(resolvedOrgID);
    setKeys(list.items);
    setError('');
  }

  async function load(): Promise<void> {
    try {
      const next = await getSession();
      setSession(next);
      const resolvedOrgID = next.auth.org_id;
      setOrgID(resolvedOrgID);
      if (next.auth.product_role === 'admin') {
        await loadKeys(resolvedOrgID);
      } else {
        setKeys([]);
        setError('');
      }
    } catch (err) {
      setSession(null);
      setError(formatFetchErrorForUser(err, t('apiKeys.error.unreachable')));
    } finally {
      setSessionLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  function selectedScopes(): string[] {
    const list: string[] = [];
    if (scopeConsoleRead) list.push(SCOPE_CONSOLE_READ);
    if (scopeConsoleWrite) list.push(SCOPE_CONSOLE_WRITE);
    return list;
  }

  async function onCreate(event: FormEvent): Promise<void> {
    event.preventDefault();
    if (!orgID || !canManage) return;
    const scopes = selectedScopes();
    if (scopes.length === 0) {
      setError(t('apiKeys.scopes.needOne'));
      return;
    }
    try {
      const resp = await createAPIKey(orgID, {
        name,
        scopes,
      });
      setNewRawKey(resp.raw_key);
      setName('');
      setError('');
      await loadKeys(orgID);
    } catch (err) {
      setError(formatFetchErrorForUser(err, t('apiKeys.error.unreachable')));
    }
  }

  async function onRotate(keyID: string): Promise<void> {
    if (!orgID || !canManage) return;
    try {
      const resp = await rotateAPIKey(orgID, keyID);
      setNewRawKey(resp.raw_key);
      await loadKeys(orgID);
    } catch (err) {
      setError(formatFetchErrorForUser(err, t('apiKeys.error.unreachable')));
    }
  }

  async function onDelete(keyID: string): Promise<void> {
    if (!orgID || !canManage) return;
    try {
      await deleteAPIKey(orgID, keyID);
      await loadKeys(orgID);
    } catch (err) {
      setError(formatFetchErrorForUser(err, t('apiKeys.error.unreachable')));
    }
  }

  if (sessionLoading) {
    return (
      <div className="page-header">
        <h1>Claves API</h1>
        <p className="text-muted">{t('apiKeys.loading')}</p>
      </div>
    );
  }

  if (!session) {
    return (
      <>
        <div className="page-header">
          <h1>Claves API</h1>
          <p>Crea y administra las claves de acceso a la API</p>
        </div>
        {error && <div className="alert alert-error">{error}</div>}
      </>
    );
  }

  if (!canManage) {
    return (
      <>
        <div className="page-header">
          <h1>Claves API</h1>
          <p>Crea y administra las claves de acceso a la API</p>
        </div>
        <div className="card">
          <div className="card-header">
            <h2>{t('apiKeys.adminOnly.title')}</h2>
          </div>
          <p>{t('apiKeys.adminOnly.body')}</p>
        </div>
      </>
    );
  }

  return (
    <>
      <div className="page-header">
        <h1>Claves API</h1>
        <p>Crea y administra las claves de acceso a la API</p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {newRawKey && (
        <div className="alert alert-warning">
          <strong>Clave generada (solo visible una vez):</strong>&nbsp;
          <code>{newRawKey}</code>
        </div>
      )}

      <div className="card">
        <div className="card-header">
          <h2>Nueva clave API</h2>
          {orgID && <span className="badge badge-neutral">Organizacion: {orgID}</span>}
        </div>
        <form onSubmit={onCreate} className="api-keys-create-form">
          <div className="form-group api-keys-field-name">
            <label>Nombre</label>
            <input placeholder="Mi clave de produccion" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <fieldset className="api-keys-scopes-fieldset">
            <legend className="api-keys-scopes-legend">{t('apiKeys.scopes.section')}</legend>
            <label className="api-keys-scope-row">
              <input
                type="checkbox"
                checked={scopeConsoleRead}
                onChange={(e) => setScopeConsoleRead(e.target.checked)}
              />
              <span className="api-keys-scope-text">
                <span className="api-keys-scope-label">{t('apiKeys.scopes.consoleRead.label')}</span>
                <span className="api-keys-scope-hint">{t('apiKeys.scopes.consoleRead.hint')}</span>
              </span>
            </label>
            <label className="api-keys-scope-row">
              <input
                type="checkbox"
                checked={scopeConsoleWrite}
                onChange={(e) => setScopeConsoleWrite(e.target.checked)}
              />
              <span className="api-keys-scope-text">
                <span className="api-keys-scope-label">{t('apiKeys.scopes.consoleWrite.label')}</span>
                <span className="api-keys-scope-hint">{t('apiKeys.scopes.consoleWrite.hint')}</span>
              </span>
            </label>
          </fieldset>
          <div className="api-keys-create-actions">
            <button type="submit" className="btn-primary">
              Crear
            </button>
          </div>
        </form>
      </div>

      <div className="card">
        <div className="card-header">
          <h2>Claves existentes</h2>
          <span className="badge badge-neutral">{keys.length}</span>
        </div>
        {keys.length === 0 ? (
          <div className="empty-state">
            <p>No hay claves API creadas</p>
          </div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Nombre</th>
                  <th>Prefijo</th>
                  <th>Permisos</th>
                  <th>Creada</th>
                  <th>Acciones</th>
                </tr>
              </thead>
              <tbody>
                {keys.map((key) => (
                  <tr key={key.id}>
                    <td className="text-semibold">{key.name}</td>
                    <td>
                      <code>{key.key_prefix}</code>
                    </td>
                    <td>
                      {key.scopes.map((s) => (
                        <span key={s} className="badge badge-neutral" title={s}>
                          {scopeBadgeLabel(s, t)}
                        </span>
                      ))}
                    </td>
                    <td className="mono">{new Date(key.created_at).toLocaleDateString()}</td>
                    <td>
                      <div className="actions-row">
                        <button className="btn-secondary btn-sm" onClick={() => void onRotate(key.id)}>
                          Rotar
                        </button>
                        <button className="btn-danger btn-sm" onClick={() => void onDelete(key.id)}>
                          Revocar
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </>
  );
}
