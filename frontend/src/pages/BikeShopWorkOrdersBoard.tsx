import { ConfiguredCrudModePage } from '../crud/configuredCrudViews';

export function BikeShopWorkOrdersBoard() {
  return <ConfiguredCrudModePage resourceId="bikeWorkOrders" modeId="kanban" />;
}

export default BikeShopWorkOrdersBoard;
