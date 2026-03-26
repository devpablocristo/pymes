import { useNavigate, useParams } from 'react-router-dom';
import { WorkOrderEditor } from '../components/WorkOrderEditor';

/**
 * Misma UI que el modal del Kanban: un solo editor de OT por ruta.
 */
export function WorkOrdersEditorPage() {
  const { orderId } = useParams<{ orderId: string }>();
  const navigate = useNavigate();

  if (!orderId) {
    return (
      <div className="card">
        <p>Falta el id de la orden.</p>
        <button type="button" className="btn btn-secondary btn-sm" onClick={() => navigate('/modules/workOrders/list')}>
          Volver a la lista
        </button>
      </div>
    );
  }

  const back = () => navigate('/modules/workOrders/list');

  return (
    <WorkOrderEditor
      variant="page"
      orderId={orderId}
      onClose={back}
      onSaved={() => {
        /* lista se refresca al volver */
      }}
    />
  );
}
