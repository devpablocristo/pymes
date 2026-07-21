import { describe, expect, it } from 'vitest';
import { appendBranchIdToCrudListQuery, isBranchScopedCrudResource } from './branchScopedCrud';

describe('branchScopedCrud', () => {
  it('marks cashflow, purchases, quotes, sales and work orders as branch-scoped resources', () => {
    expect(isBranchScopedCrudResource('cashflow')).toBe(true);
    expect(isBranchScopedCrudResource('purchases')).toBe(true);
    expect(isBranchScopedCrudResource('quotes')).toBe(true);
    expect(isBranchScopedCrudResource('sales')).toBe(true);
    expect(isBranchScopedCrudResource('carWorkOrders')).toBe(true);
    expect(isBranchScopedCrudResource('bikeWorkOrders')).toBe(true);
    expect(isBranchScopedCrudResource('customers')).toBe(false);
  });

  it('appends branch_id only for branch-scoped resources with a selected branch', () => {
    const cashflowQuery = appendBranchIdToCrudListQuery('cashflow', new URLSearchParams('limit=20'), 'branch-a');
    expect(cashflowQuery.get('branch_id')).toBe('branch-a');

    const purchasesQuery = appendBranchIdToCrudListQuery('purchases', new URLSearchParams('limit=20'), 'branch-a');
    expect(purchasesQuery.get('branch_id')).toBe('branch-a');

    const quotesQuery = appendBranchIdToCrudListQuery('quotes', new URLSearchParams('limit=20'), 'branch-a');
    expect(quotesQuery.get('branch_id')).toBe('branch-a');

    const salesQuery = appendBranchIdToCrudListQuery('sales', new URLSearchParams('limit=20'), 'branch-a');
    expect(salesQuery.get('branch_id')).toBe('branch-a');

    const customersQuery = appendBranchIdToCrudListQuery('customers', new URLSearchParams('limit=20'), 'branch-a');
    expect(customersQuery.get('branch_id')).toBeNull();

    const missingBranchQuery = appendBranchIdToCrudListQuery('sales', new URLSearchParams('limit=20'), null);
    expect(missingBranchQuery.get('branch_id')).toBeNull();
  });
});
