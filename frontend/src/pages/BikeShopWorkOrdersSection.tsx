import { ConfiguredCrudSection } from '../crud/configuredCrudViews';

export function BikeShopWorkOrdersSection() {
  return <ConfiguredCrudSection resourceId="bikeWorkOrders" baseRoute="/workshops/bike-shop/orders" />;
}

export default BikeShopWorkOrdersSection;
