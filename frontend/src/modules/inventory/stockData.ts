import { apiRequest } from '../../lib/api';
import { readActiveBranchId } from '../../lib/branchSelectionStorage';

export type StockLevelRow = {
  id: string;
  product_id: string;
  product_name: string;
  sku: string;
  quantity: number;
  min_quantity: number;
  track_stock?: boolean;
  is_low_stock: boolean;
  updated_at: string;
  created_by?: string;
};

export function mapInventoryItem(row: Omit<StockLevelRow, 'id'>): StockLevelRow {
  return { ...row, id: row.product_id };
}

export async function fetchStockLevels(opts?: { archived?: boolean }): Promise<StockLevelRow[]> {
  const query = new URLSearchParams({ limit: '500' });
  if (opts?.archived) query.set('archived', 'true');
  const branchId = readActiveBranchId();
  if (branchId) query.set('branch_id', branchId);
  const data = await apiRequest<{ items?: Array<Omit<StockLevelRow, 'id'>> | null }>(`/v1/inventory?${query.toString()}`);
  return (data.items ?? []).map(mapInventoryItem);
}

export async function fetchStockLevelByProductId(productId: string): Promise<StockLevelRow> {
  const query = new URLSearchParams();
  const branchId = readActiveBranchId();
  if (branchId) query.set('branch_id', branchId);
  const suffix = query.size > 0 ? `?${query.toString()}` : '';
  const row = await apiRequest<Omit<StockLevelRow, 'id'>>(`/v1/inventory/${encodeURIComponent(productId)}${suffix}`);
  return mapInventoryItem(row);
}
