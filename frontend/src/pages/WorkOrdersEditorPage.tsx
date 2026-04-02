import { useNavigate, useParams } from 'react-router-dom';
import { PageLayout } from '../components/PageLayout';
import { WorkOrderEditor } from '../components/WorkOrderEditor';
import { useI18n } from '../lib/i18n';
import { WorkOrdersHeaderLead } from './WorkOrdersHeaderLead';

/**
 * Misma UI que el modal del Kanban: un solo editor de OT por ruta.
 */
export function WorkOrdersEditorPage() {
  const { orderId } = useParams<{ orderId: string }>();
  const navigate = useNavigate();
  const { t } = useI18n();

  const content = !orderId ? (
    <div className="card">
      <p>Falta el id de la orden.</p>
      <button type="button" className="btn btn-secondary btn-sm" onClick={() => navigate('/modules/workOrders/list')}>
        Volver a la lista
      </button>
    </div>
  ) : (
    <WorkOrderEditor
      variant="page"
      orderId={orderId}
      onClose={() => navigate('/modules/workOrders/list')}
      onSaved={() => {
        /* lista se refresca al volver */
      }}
    />
  );

  return (
    <PageLayout className="wo-mod-orders" title={t('shell.workOrders.pageTitle')} lead={<WorkOrdersHeaderLead />}>
      {content}
    </PageLayout>
  );
}
