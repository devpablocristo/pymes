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
    <div className="card">
      <h1>Dashboard</h1>
      {error && <p style={{ color: 'crimson' }}>{error}</p>}
      <p>Org actual: {orgID || 'N/A'}</p>
      <pre>{JSON.stringify(me, null, 2)}</pre>
    </div>
  );
}
