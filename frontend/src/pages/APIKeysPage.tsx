import { FormEvent, useEffect, useState } from 'react';
import { createAPIKey, deleteAPIKey, getAPIKeys, getSession, rotateAPIKey } from '../lib/api';
import { useI18n } from '../lib/i18n';
import type { APIKeyItem, SessionResponse } from '../lib/types';

export function APIKeysPage() {
  const { t } = useI18n();
  const [sessionLoading, setSessionLoading] = useState(true);
  const [session, setSession] = useState<SessionResponse | null>(null);
  const [orgID, setOrgID] = useState('');
  const [keys, setKeys] = useState<APIKeyItem[]>([]);
  const [name, setName] = useState('');
  const [scopes, setScopes] = useState('admin:console:read,admin:console:write');
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
      setError(String(err));
    } finally {
      setSessionLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function onCreate(event: FormEvent): Promise<void> {
    event.preventDefault();
    if (!orgID || !canManage) return;
    try {
      const resp = await createAPIKey(orgID, {
        name,
        scopes: scopes
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
      });
      setNewRawKey(resp.raw_key);
      setName('');
      await loadKeys(orgID);
    } catch (err) {
      setError(String(err));
    }
  }

  async function onRotate(keyID: string): Promise<void> {
    if (!orgID || !canManage) return;
    try {
      const resp = await rotateAPIKey(orgID, keyID);
      setNewRawKey(resp.raw_key);
      await loadKeys(orgID);
    } catch (err) {
      setError(String(err));
    }
  }

  async function onDelete(keyID: string): Promise<void> {
    if (!orgID || !canManage) return;
    try {
      await deleteAPIKey(orgID, keyID);
      await loadKeys(orgID);
    } catch (err) {
      setError(String(err));
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
        <form onSubmit={onCreate} className="form-row">
          <div className="form-group grow">
            <label>Nombre</label>
            <input placeholder="Mi clave de produccion" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="form-group grow">
            <label>Permisos</label>
            <input value={scopes} onChange={(e) => setScopes(e.target.value)} />
          </div>
          <button type="submit" className="btn-primary">
            Crear
          </button>
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
                        <span key={s} className="badge badge-neutral">
                          {s}
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
