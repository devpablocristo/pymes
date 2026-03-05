import { useEffect, useState } from 'react';
import { createCheckout, createPortal, getBillingStatus } from '../lib/api';
import type { BillingStatus } from '../lib/types';

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
    <div className="card">
      <h1>Billing</h1>
      {error && <p style={{ color: 'crimson' }}>{error}</p>}
      <div className="row">
        <button onClick={() => void upgrade('growth')}>Upgrade a Growth</button>
        <button onClick={() => void upgrade('enterprise')}>Upgrade a Enterprise</button>
        <button className="secondary" onClick={() => void openPortal()}>
          Manage billing
        </button>
      </div>
      <pre>{JSON.stringify(status, null, 2)}</pre>
    </div>
  );
}
