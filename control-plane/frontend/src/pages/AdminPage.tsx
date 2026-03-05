import { FormEvent, useEffect, useState } from 'react';
import { getAuditEntries, getTenantSettings, updateTenantSettings } from '../lib/api';
import type { TenantSettings } from '../lib/types';

export function AdminPage() {
  const [settings, setSettings] = useState<TenantSettings | null>(null);
  const [planCode, setPlanCode] = useState('starter');
  const [activity, setActivity] = useState<unknown[]>([]);
  const [error, setError] = useState('');

  async function load(): Promise<void> {
    try {
      const [tenant, audit] = await Promise.all([getTenantSettings(), getAuditEntries()]);
      setSettings(tenant);
      setPlanCode(tenant.plan_code);
      setActivity(audit.items);
      setError('');
    } catch (err) {
      setError(String(err));
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function onSubmit(event: FormEvent): Promise<void> {
    event.preventDefault();
    try {
      const updated = await updateTenantSettings({ plan_code: planCode });
      setSettings(updated);
      setError('');
    } catch (err) {
      setError(String(err));
    }
  }

  return (
    <div className="card">
      <h1>Admin</h1>
      {error && <p style={{ color: 'crimson' }}>{error}</p>}
      <form onSubmit={onSubmit} className="row" style={{ alignItems: 'center' }}>
        <label>Plan</label>
        <select value={planCode} onChange={(e) => setPlanCode(e.target.value)}>
          <option value="starter">starter</option>
          <option value="growth">growth</option>
          <option value="enterprise">enterprise</option>
        </select>
        <button type="submit">Actualizar settings</button>
      </form>
      <h3>Tenant settings</h3>
      <pre>{JSON.stringify(settings, null, 2)}</pre>
      <h3>Audit (últimos eventos)</h3>
      <pre>{JSON.stringify(activity.slice(0, 20), null, 2)}</pre>
    </div>
  );
}
