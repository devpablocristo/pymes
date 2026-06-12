import { useQuery } from '@tanstack/react-query';
import { apiRequest } from '../../lib/api';
import { useOptionalBranchSelection } from '../../lib/useBranchSelection';
import { readActiveBranchId } from '../../lib/branchSelectionStorage';

export function useDashboardDataEndpoint<T>(dataEndpoint: string, context: string) {
  const branchSelection = useOptionalBranchSelection();
  const branchId = branchSelection?.selectedBranchId ?? readActiveBranchId();

  return useQuery({
    queryKey: ['dashboard-data', context, dataEndpoint, branchId],
    queryFn: () => apiRequest<T>(withContext(dataEndpoint, context, branchId)),
    staleTime: 30_000,
    retry: 1,
  });
}

function withContext(path: string, context: string, branchId: string | null | undefined): string {
  const separator = path.includes('?') ? '&' : '?';
  const base = `${path}${separator}context=${encodeURIComponent(context)}`;
  const normalizedBranchId = branchId?.trim();
  if (!normalizedBranchId) {
    return base;
  }
  return `${base}&branch_id=${encodeURIComponent(normalizedBranchId)}`;
}
