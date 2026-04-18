// URL slug canónico para recursos CRUD. Los resourceId internos (keys en
// `resourceConfigs` + `crudModuleCatalog`) siguen en camelCase porque son
// identificadores de código; las URLs que consume el usuario usan kebab-case.

const CAMEL_TO_SLUG: Record<string, string> = {
  bikeWorkOrders: 'bike-work-orders',
  carWorkOrders: 'car-work-orders',
  workshopVehicles: 'workshop-vehicles',
  restaurantDiningAreas: 'restaurant-dining-areas',
  restaurantDiningTables: 'restaurant-dining-tables',
  creditNotes: 'credit-notes',
  priceLists: 'price-lists',
  procurementRequests: 'procurement-requests',
  procurementPolicies: 'procurement-policies',
};

const SLUG_TO_CAMEL: Record<string, string> = Object.fromEntries(
  Object.entries(CAMEL_TO_SLUG).map(([camel, slug]) => [slug, camel]),
);

/** camelCase → kebab-case para URLs. Recursos sin mapeo se devuelven tal cual. */
export function toCrudResourceSlug(resourceId: string): string {
  return CAMEL_TO_SLUG[resourceId] ?? resourceId;
}

/** kebab-case desde URL → camelCase resourceId interno. */
export function fromCrudResourceSlug(slug: string): string {
  return SLUG_TO_CAMEL[slug] ?? slug;
}
