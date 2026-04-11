import { ConfiguredCrudModePage } from '../crud/configuredCrudViews';

export function WorkOrdersKanbanPanel() {
  return <ConfiguredCrudModePage resourceId="carWorkOrders" modeId="kanban" />;
}
