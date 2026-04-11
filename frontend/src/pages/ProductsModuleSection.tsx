import { ConfiguredCrudSection } from '../crud/configuredCrudViews';

export function ProductsModuleSection() {
  return <ConfiguredCrudSection resourceId="products" baseRoute="/modules/products" />;
}
