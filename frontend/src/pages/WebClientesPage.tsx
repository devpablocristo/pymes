import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { PublicSchedulingFlow, createPublicSchedulingClient } from '@devpablocristo/modules-scheduling';
import '@devpablocristo/modules-scheduling/styles.css';
import { PageLayout } from '../components/PageLayout';
import { getSession, apiRequest } from '../lib/api';
import { queryKeys } from '../lib/queryKeys';
import { useI18n } from '../lib/i18n';

const publicSchedulingClient = createPublicSchedulingClient(apiRequest);

/**
 * Flujo de reserva como lo vive el cliente final (PublicSchedulingFlow), embebido en consola
 * para prueba y soporte. La operación interna (calendario + cola) vive en /agenda.
 */
export function WebClientesPage() {
  const { language, t } = useI18n();
  const [orgRef, setOrgRef] = useState('');
  const [activeOrgRef, setActiveOrgRef] = useState('');
  const orgInputId = 'web-clientes-org-ref';

  const sessionQuery = useQuery({
    queryKey: queryKeys.session.current,
    queryFn: getSession,
  });

  useEffect(() => {
    const orgId = sessionQuery.data?.auth.org_id?.trim();
    if (!orgId) {
      return;
    }
    setOrgRef((current) => current || orgId);
    setActiveOrgRef((current) => current || orgId);
  }, [sessionQuery.data]);

  function handleLoad() {
    if (orgRef.trim()) {
      setActiveOrgRef(orgRef.trim());
    }
  }

  const error = sessionQuery.error instanceof Error ? sessionQuery.error.message : '';

  return (
    <PageLayout title={t('calendar.webClientes.title')} lead={t('calendar.webClientes.lead')}>
      {error && <div className="alert alert-error">{error}</div>}

      <div className="card u-mb-md">
        <div className="card-header">
          <h2>{t('calendar.webClientes.configTitle')}</h2>
        </div>
        <div className="form-row">
          <div className="form-group grow">
            <label htmlFor={orgInputId}>{t('calendar.webClientes.orgRefLabel')}</label>
            <input
              id={orgInputId}
              value={orgRef}
              onChange={(e) => setOrgRef(e.target.value)}
              placeholder={t('calendar.webClientes.orgRefPlaceholder')}
            />
          </div>
          <button type="button" className="btn-primary" onClick={handleLoad} disabled={!orgRef.trim()}>
            {t('calendar.webClientes.load')}
          </button>
        </div>
      </div>

      <div className="alert alert-warning u-mb-md">{t('calendar.webClientes.notice')}</div>

      <PublicSchedulingFlow
        client={publicSchedulingClient}
        orgRef={activeOrgRef}
        locale={language === 'en' ? 'en' : 'es'}
      />
    </PageLayout>
  );
}

export default WebClientesPage;
