import { ConfiguredCrudSection } from '../crud/configuredCrudViews';

export function StockModuleSection() {
  return (
    <ConfiguredCrudSection
      resourceId="stock"
      baseRoute="/modules/stock"
      actionLink={{
        to: '/modules/stock/configure',
        label: 'Configurar',
        hideWhenActivePattern: '/modules/stock/configure',
        activeReplacement: {
          to: '/modules/stock/list',
          label: 'Volver al inventario',
        },
      }}
    />
  );
}

export default StockModuleSection;
