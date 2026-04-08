import { LazyConfiguredCrudPage } from '../crud/lazyCrudPage';

/** Vista tabla: CRUD canónico sin wrapper de modo (el switch vive en ProductsModuleSection). */
export function ProductsListPage() {
  return <LazyConfiguredCrudPage resourceId="products" />;
}
