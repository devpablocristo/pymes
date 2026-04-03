/**
 * Header lead específico de auto-repair (delega al genérico con paths fijos).
 */
import { WorkOrdersHeaderLead as GenericHeaderLead } from '../components/WorkOrdersHeaderLead';

export function WorkOrdersHeaderLead() {
  return (
    <GenericHeaderLead
      boardPath="/modules/workOrders/board"
      listPath="/modules/workOrders/list"
      editPattern="/modules/workOrders/edit/:orderId"
    />
  );
}
