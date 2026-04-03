import { useQuery } from '@tanstack/react-query';
import { apiRequest } from '../../lib/api';
import type { DashboardContext, DashboardWidgetDefinition } from '../types';

export function useDashboardWidgetData<T>(widget: DashboardWidgetDefinition, context: DashboardContext) {
  return useQuery({
    queryKey: ['dashboard-widget', context, widget.widget_key],
    queryFn: () => apiRequest<T>(withContext(widget.data_endpoint, context)),
    staleTime: 30_000,
    retry: 1,
  });
}

function withContext(path: string, context: DashboardContext): string {
  const separator = path.includes('?') ? '&' : '?';
  return `${path}${separator}context=${encodeURIComponent(String(context))}`;
}
