import { FormEvent, useEffect, useState } from 'react';
import { createAPIKey, deleteAPIKey, getAdminBootstrap, getAPIKeys, rotateAPIKey } from '../lib/api';
import type { APIKeyItem } from '../lib/types';

export function APIKeysPage() {
  const [orgID, setOrgID] = useState('');
  const [keys, setKeys] = useState<APIKeyItem[]>([]);
  const [name, setName] = useState('');
  const [scopes, setScopes] = useState('admin:console:read,admin:console:write');
  const [newRawKey, setNewRawKey] = useState('');
  const [error, setError] = useState('');

  async function load(): Promise<void> {
    try {
      const bootstrap = await getAdminBootstrap();
      const resolvedOrgID = bootstrap.settings.org_id;
      setOrgID(resolvedOrgID);
      const list = await getAPIKeys(resolvedOrgID);
      setKeys(list.items);
      setError('');
    } catch (err) {
      setError(String(err));
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function onCreate(event: FormEvent): Promise<void> {
    event.preventDefault();
    if (!orgID) return;
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
      await load();
    } catch (err) {
      setError(String(err));
    }
  }

  async function onRotate(keyID: string): Promise<void> {
    if (!orgID) return;
    try {
      const resp = await rotateAPIKey(orgID, keyID);
      setNewRawKey(resp.raw_key);
      await load();
    } catch (err) {
      setError(String(err));
    }
  }

  async function onDelete(keyID: string): Promise<void> {
    if (!orgID) return;
    try {
      await deleteAPIKey(orgID, keyID);
      await load();
    } catch (err) {
      setError(String(err));
    }
  }

  return (
    <div className="card">
      <h1>API Keys</h1>
      {error && <p style={{ color: 'crimson' }}>{error}</p>}
      <p>Org: {orgID || 'N/A'}</p>
      <form onSubmit={onCreate} className="row">
        <input placeholder="Nombre" value={name} onChange={(e) => setName(e.target.value)} />
        <input value={scopes} onChange={(e) => setScopes(e.target.value)} style={{ minWidth: 360 }} />
        <button type="submit">Crear API key</button>
      </form>
      {newRawKey && (
        <div className="card" style={{ marginTop: '1rem', borderColor: '#ffc857' }}>
          <strong>Raw key (solo visible una vez):</strong>
          <pre>{newRawKey}</pre>
        </div>
      )}
      <table style={{ width: '100%', marginTop: '1rem' }}>
        <thead>
          <tr>
            <th align="left">Name</th>
            <th align="left">Prefix</th>
            <th align="left">Scopes</th>
            <th align="left">Actions</th>
          </tr>
        </thead>
        <tbody>
          {keys.map((key) => (
            <tr key={key.id}>
              <td>{key.name}</td>
              <td>{key.key_prefix}</td>
              <td>{key.scopes.join(', ')}</td>
              <td className="row">
                <button className="secondary" onClick={() => void onRotate(key.id)}>
                  Rotar
                </button>
                <button onClick={() => void onDelete(key.id)}>Revocar</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
