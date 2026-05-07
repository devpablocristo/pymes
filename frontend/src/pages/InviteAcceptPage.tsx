import { useClerk, useSession } from '@clerk/react';
import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { acceptTenantInvite } from '../lib/api';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { queryKeys } from '../lib/queryKeys';

export function InviteAcceptPage() {
  const [params] = useSearchParams();
  const token = params.get('token')?.trim() ?? '';
  const clerk = useClerk();
  const { session } = useSession();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<'loading' | 'done' | 'error'>('loading');
  const [error, setError] = useState('');

  useEffect(() => {
    let alive = true;
    async function run() {
      if (!token) {
        setStatus('error');
        setError('La invitación no tiene token.');
        return;
      }
      try {
        const accepted = await acceptTenantInvite(token);
        if (accepted.clerk_org_id) {
          await clerk.setActive({ organization: accepted.clerk_org_id });
          await session?.reload();
        }
        await Promise.all([
          queryClient.invalidateQueries({ queryKey: queryKeys.session.current }),
          queryClient.invalidateQueries({ queryKey: queryKeys.me.current }),
          queryClient.invalidateQueries({ queryKey: queryKeys.tenant.settings }),
        ]);
        if (!alive) return;
        setStatus('done');
        navigate('/', { replace: true });
      } catch (err) {
        if (!alive) return;
        setStatus('error');
        setError(formatFetchErrorForUser(err, 'No se pudo aceptar la invitación.'));
      }
    }
    void run();
    return () => {
      alive = false;
    };
  }, [clerk, navigate, queryClient, session, token]);

  return (
    <main className="auth-page">
      <section className="card auth-card">
        <h1>Invitación</h1>
        {status === 'loading' ? <p>Validando invitación…</p> : null}
        {status === 'done' ? <p>Invitación aceptada.</p> : null}
        {status === 'error' ? <p className="form-error">{error}</p> : null}
      </section>
    </main>
  );
}
