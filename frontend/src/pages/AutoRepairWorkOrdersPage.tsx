import { LazyConfiguredCrudPage } from '../crud/lazyCrudPage';

export function AutoRepairWorkOrdersPage() {
  return <LazyConfiguredCrudPage resourceId="workOrders" />;
}
