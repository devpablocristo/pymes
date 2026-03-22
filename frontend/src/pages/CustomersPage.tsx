import { LazyConfiguredCrudPage } from '../crud/lazyCrudPage';

export function CustomersPage() {
  return <LazyConfiguredCrudPage resourceId="customers" />;
}
