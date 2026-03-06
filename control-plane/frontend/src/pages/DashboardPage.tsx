import { Link } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { apiRequest, getAdminBootstrap, getMe } from '../lib/api';
import { moduleGroups, moduleList } from '../lib/moduleCatalog';

export function DashboardPage() {
  const [me, setMe] = useState<Record<string, unknown> | null>(null);
  const [orgID, setOrgID] = useState<string>('');
  const [dashboard, setDashboard] = useState<Record<string, unknown> | null>(null);
  const [error, setError] = useState<string>('');

  useEffect(() => {
    (async () => {
      try {
        const [meResp, bootstrap] = await Promise.all([getMe(), getAdminBootstrap()]);
        setMe(meResp);
        setOrgID(bootstrap.settings.org_id);
        try {
          const dashboardResp = await apiRequest<Record<string, unknown>>('/v1/dashboard');
          setDashboard(dashboardResp);
        } catch {
          setDashboard(null);
        }
      } catch (err) {
        setError(String(err));
      }
    })();
  }, []);

  return (
    <>
      <div className="page-header">
        <h1>Panel</h1>
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
          <div className="stat-label">Estado</div>
          <div className="stat-value">
            {me ? <span className="badge badge-success">Activo</span> : <span className="badge badge-neutral">---</span>}
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Modulos FE</div>
          <div className="stat-value">{moduleList.length + 6}</div>
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <h2>Cobertura funcional</h2>
          <span className="badge badge-neutral">{moduleList.length} modulos enroutados</span>
        </div>
        <div className="module-link-grid">
          {moduleGroups.map((group) => (
            <div key={group.id} className="module-link-card">
              <span className="sidebar-token">{group.label.slice(0, 2).toUpperCase()}</span>
              <div>
                <strong>{group.label}</strong>
                <p>
                  {moduleList
                    .filter((module) => module.group === group.id)
                    .slice(0, 5)
                    .map((module) => module.navLabel)
                    .join(', ')}
                </p>
              </div>
            </div>
          ))}
        </div>
        <div className="actions-row" style={{ marginTop: '1rem', flexWrap: 'wrap' }}>
          {moduleList.slice(0, 8).map((module) => (
            <Link key={module.id} to={`/modules/${module.id}`} className="btn-secondary btn-sm">
              {module.navLabel}
            </Link>
          ))}
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

      {dashboard && (
        <div className="card">
          <div className="card-header">
            <h2>Dashboard operativo</h2>
          </div>
          <pre>{JSON.stringify(dashboard, null, 2)}</pre>
        </div>
      )}
    </>
  );
}
