import { useQuery } from '@tanstack/react-query';
import { apiRequest } from '../../lib/api';

export function useDashboardDataEndpoint<T>(dataEndpoint: string, context: string) {
  return useQuery({
    queryKey: ['dashboard-data', context, dataEndpoint],
    queryFn: () => apiRequest<T>(withContext(dataEndpoint, context)),
    staleTime: 30_000,
    retry: 1,
  });
}

function withContext(path: string, context: string): string {
  const separator = path.includes('?') ? '&' : '?';
  return `${path}${separator}context=${encodeURIComponent(context)}`;
}
