import { useEffect, useState } from 'react';
import { getAdminBootstrap, getMe } from '../lib/api';

export function DashboardPage() {
  const [me, setMe] = useState<Record<string, unknown> | null>(null);
  const [orgID, setOrgID] = useState<string>('');
  const [error, setError] = useState<string>('');

  useEffect(() => {
    (async () => {
      try {
        const [meResp, bootstrap] = await Promise.all([getMe(), getAdminBootstrap()]);
        setMe(meResp);
        setOrgID(bootstrap.settings.org_id);
      } catch (err) {
        setError(String(err));
      }
    })();
  }, []);

  return (
    <>
      <div className="page-header">
        <h1>Dashboard</h1>
        <p>Vista general de tu cuenta y organizacion</p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="stats-grid">
        <div className="stat-card">
          <div className="stat-label">Organizacion</div>
          <div className="stat-value mono">{orgID || '---'}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Usuario</div>
          <div className="stat-value mono">
            {me ? String((me as Record<string, unknown>).email ?? (me as Record<string, unknown>).id ?? '---') : '---'}
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Status</div>
          <div className="stat-value">
            {me ? <span className="badge badge-success">Activo</span> : <span className="badge badge-neutral">---</span>}
          </div>
        </div>
      </div>

      {me && (
        <div className="card">
          <div className="card-header">
            <h2>Datos del usuario</h2>
          </div>
          <pre>{JSON.stringify(me, null, 2)}</pre>
        </div>
      )}
    </>
  );
}
