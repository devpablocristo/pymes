import { ConfiguredCrudSection } from '../crud/configuredCrudViews';

export function WorkOrdersModuleSection() {
  return (
    <ConfiguredCrudSection
      resourceId="carWorkOrders"
      baseRoute="/modules/carWorkOrders"
      contextPatternByModeId={{ list: '/modules/carWorkOrders/edit/:orderId' }}
    />
  );
}
