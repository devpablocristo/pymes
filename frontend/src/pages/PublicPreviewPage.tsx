import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { PublicSchedulingFlow, createPublicSchedulingClient } from '@devpablocristo/modules-scheduling';
import '@devpablocristo/modules-scheduling/styles.css';
import { PageLayout } from '../components/PageLayout';
import { getSession, apiRequest } from '../lib/api';
import { queryKeys } from '../lib/queryKeys';
import { useI18n } from '../lib/i18n';

const publicSchedulingClient = createPublicSchedulingClient(apiRequest);

export function PublicPreviewPage() {
  const { language } = useI18n();
  const [orgRef, setOrgRef] = useState('');
  const [activeOrgRef, setActiveOrgRef] = useState('');
  const orgInputId = 'public-preview-org-ref';

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
    <PageLayout title="Reserva pública" lead="Vista previa del flujo público de scheduling">
      {error && <div className="alert alert-error">{error}</div>}

      <div className="card u-mb-md">
        <div className="card-header">
          <h2>Configuración</h2>
        </div>
        <div className="form-row">
          <div className="form-group grow">
            <label htmlFor={orgInputId}>Referencia de organización</label>
            <input
              id={orgInputId}
              value={orgRef}
              onChange={(e) => setOrgRef(e.target.value)}
              placeholder="slug-publico-o-uuid"
            />
          </div>
          <button type="button" className="btn-primary" onClick={handleLoad} disabled={!orgRef.trim()}>
            Cargar
          </button>
        </div>
      </div>

      <div className="alert alert-warning u-mb-md">
        Esta es una vista previa. La página pública real se sirve desde una URL separada y usa el mismo contrato público.
      </div>

      <PublicSchedulingFlow
        client={publicSchedulingClient}
        orgRef={activeOrgRef}
        locale={language === 'en' ? 'en' : 'es'}
      />
    </PageLayout>
  );
}
