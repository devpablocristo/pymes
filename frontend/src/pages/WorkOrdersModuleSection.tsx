import { ConfiguredCrudSection } from '../crud/configuredCrudViews';

export function WorkOrdersModuleSection() {
  return (
    <ConfiguredCrudSection
      resourceId="carWorkOrders"
      baseRoute="/modules/carWorkOrders"
      secondaryContextPattern="/modules/carWorkOrders/edit/:orderId"
    />
  );
}
