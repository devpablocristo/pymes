// URL slug canónico para recursos CRUD. Los resourceId internos (keys en
// `resourceConfigs` + `crudModuleCatalog`) siguen en camelCase porque son
// identificadores de código; las URLs que consume el usuario usan kebab-case.
//
// Ola B5: la mecánica genérica (camelToKebab / kebabToCamel / applySlugMap)
// vive en @devpablocristo/platform-browser. Acá sólo definimos el mapping
// pymes-specific para resourceIds cuya transformación algorítmica no
// produce el slug deseado (irregularidades en pymes).

import { applySlugMap, unapplySlugMap } from '@devpablocristo/platform-browser';

const CAMEL_TO_SLUG: Record<string, string> = {
  bikeWorkOrders: 'bike-work-orders',
  carWorkOrders: 'car-work-orders',
  workshopVehicles: 'workshop-vehicles',
  restaurantDiningAreas: 'restaurant-dining-areas',
  restaurantDiningTables: 'restaurant-dining-tables',
  creditNotes: 'credit-notes',
  priceLists: 'price-lists',
  procurementRequests: 'procurement-requests',
};

/** camelCase → kebab-case para URLs. Recursos sin mapeo se devuelven tal cual. */
export function toCrudResourceSlug(resourceId: string): string {
  return applySlugMap(resourceId, CAMEL_TO_SLUG);
}

/** kebab-case desde URL → camelCase resourceId interno. */
export function fromCrudResourceSlug(slug: string): string {
  return unapplySlugMap(slug, CAMEL_TO_SLUG);
}
