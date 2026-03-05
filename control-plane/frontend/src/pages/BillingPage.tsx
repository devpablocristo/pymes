import { useEffect, useState } from 'react';
import { createCheckout, createPortal, getBillingStatus } from '../lib/api';
import type { BillingStatus } from '../lib/types';

const plans = [
  { code: 'starter', name: 'Starter', description: 'Para equipos pequenos' },
  { code: 'growth', name: 'Growth', description: 'Para empresas en crecimiento' },
  { code: 'enterprise', name: 'Enterprise', description: 'Para grandes organizaciones' },
];

export function BillingPage() {
  const [status, setStatus] = useState<BillingStatus | null>(null);
  const [error, setError] = useState('');

  async function load(): Promise<void> {
    try {
      const resp = await getBillingStatus();
      setStatus(resp);
      setError('');
    } catch (err) {
      setError(String(err));
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function upgrade(planCode: string): Promise<void> {
    try {
      const resp = await createCheckout({
        plan_code: planCode,
        success_url: `${window.location.origin}/billing`,
        cancel_url: `${window.location.origin}/billing`,
      });
      window.open(resp.checkout_url, '_blank');
    } catch (err) {
      setError(String(err));
    }
  }

  async function openPortal(): Promise<void> {
    try {
      const resp = await createPortal({ return_url: `${window.location.origin}/billing` });
      window.open(resp.portal_url, '_blank');
    } catch (err) {
      setError(String(err));
    }
  }

  return (
    <>
      <div className="page-header">
        <h1>Billing</h1>
        <p>Gestiona tu plan y metodo de pago</p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {status && (
        <div className="stats-grid">
          <div className="stat-card">
            <div className="stat-label">Plan actual</div>
            <div className="stat-value" style={{ textTransform: 'capitalize' }}>{status.plan_code}</div>
          </div>
          <div className="stat-card">
            <div className="stat-label">Estado</div>
            <div className="stat-value">
              <span className={`badge ${status.status === 'active' ? 'badge-success' : 'badge-warning'}`}>
                {status.status}
              </span>
            </div>
          </div>
          <div className="stat-card">
            <div className="stat-label">Fin del periodo</div>
            <div className="stat-value mono" style={{ fontSize: '1rem' }}>
              {status.current_period_end ? new Date(status.current_period_end).toLocaleDateString() : '---'}
            </div>
          </div>
        </div>
      )}

      <div className="card">
        <div className="card-header">
          <h2>Planes disponibles</h2>
          <button className="btn-secondary btn-sm" onClick={() => void openPortal()}>
            Gestionar pagos
          </button>
        </div>
        <div className="plans-grid">
          {plans.map((plan) => (
            <div
              key={plan.code}
              className={`plan-card${status?.plan_code === plan.code ? ' current' : ''}`}
            >
              <h3>{plan.name}</h3>
              <p style={{ color: 'var(--color-text-secondary)', fontSize: '0.85rem' }}>
                {plan.description}
              </p>
              <div className="plan-badge">
                {status?.plan_code === plan.code ? (
                  <span className="badge badge-success">Plan actual</span>
                ) : (
                  <button
                    className="btn-primary btn-sm"
                    onClick={() => void upgrade(plan.code)}
                  >
                    Upgrade
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}
