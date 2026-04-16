const BRANCH_SCOPED_CRUD_RESOURCES = new Set<string>([
  'cashflow',
  'purchases',
  'quotes',
  'sales',
  'carWorkOrders',
  'bikeWorkOrders',
]);

export function isBranchScopedCrudResource(resourceId: string): boolean {
  return BRANCH_SCOPED_CRUD_RESOURCES.has(resourceId);
}

export function appendBranchIdToCrudListQuery(
  resourceId: string,
  query: URLSearchParams,
  branchId: string | null | undefined,
): URLSearchParams {
  if (!isBranchScopedCrudResource(resourceId)) {
    return query;
  }
  const normalized = branchId?.trim();
  if (!normalized) {
    return query;
  }
  query.set('branch_id', normalized);
  return query;
}
