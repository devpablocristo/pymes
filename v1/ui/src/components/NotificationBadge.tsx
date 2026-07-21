import { useQuery } from '@tanstack/react-query';
import { getNotificationsSummary } from '../lib/api';
import { queryKeys } from '../lib/queryKeys';

const POLL_MS = 60_000;

export function NotificationBadge() {
  const { data } = useQuery({
    queryKey: queryKeys.notifications.summary,
    queryFn: getNotificationsSummary,
    refetchInterval: POLL_MS,
  });
  const count = data?.unread_count ?? 0;
  if (count === 0) return null;
  return (
    <span
      style={{
        backgroundColor: '#547792',
        color: '#fff',
        fontSize: '10px',
        fontWeight: 600,
        borderRadius: '9999px',
        padding: '1px 6px',
        marginLeft: '6px',
        lineHeight: '16px',
        display: 'inline-block',
        minWidth: '18px',
        textAlign: 'center',
      }}
    >
      {count > 99 ? '99+' : count}
    </span>
  );
}
