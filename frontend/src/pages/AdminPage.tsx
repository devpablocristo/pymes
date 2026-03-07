import { FormEvent, useEffect, useState } from 'react';
import { getAuditEntries, getTenantSettings, updateTenantSettings } from '../lib/api';
import type { TenantSettings } from '../lib/types';

const planLabels: Record<string, string> = {
  starter: 'Inicial',
  growth: 'Crecimiento',
  enterprise: 'Empresas',
};

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
    <>
      <div className="page-header">
        <h1>Administracion</h1>
        <p>Configuración del espacio y registro de actividad</p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="card">
        <div className="card-header">
          <h2>Configuración del espacio</h2>
        </div>
        <form onSubmit={onSubmit} className="form-row">
          <div className="form-group">
            <label>Plan</label>
            <select value={planCode} onChange={(e) => setPlanCode(e.target.value)}>
              <option value="starter">{planLabels.starter}</option>
              <option value="growth">{planLabels.growth}</option>
              <option value="enterprise">{planLabels.enterprise}</option>
            </select>
          </div>
          <button type="submit" className="btn-primary">Actualizar</button>
        </form>
        {settings && (
          <pre style={{ marginTop: '1rem' }}>{JSON.stringify(settings, null, 2)}</pre>
        )}
      </div>

      <div className="card">
        <div className="card-header">
          <h2>Registro de auditoria</h2>
          <span className="badge badge-neutral">{activity.length} eventos</span>
        </div>
        {activity.length === 0 ? (
          <div className="empty-state">
            <p>Sin eventos registrados</p>
          </div>
        ) : (
          <pre>{JSON.stringify(activity.slice(0, 20), null, 2)}</pre>
        )}
      </div>
    </>
  );
}
