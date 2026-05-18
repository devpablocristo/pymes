import { useCallback } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { PageLayout } from '../components/PageLayout';
import { usePageSearch } from '../components/PageSearch';
import { useSearch } from '@devpablocristo/modules-search';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { useI18n } from '../lib/i18n';
import { getNotificationPreferences, updateNotificationPreference } from '../lib/api';
import { queryKeys } from '../lib/queryKeys';
import type { NotificationPreference } from '../lib/types';

function labelForType(code: string, t: (key: string, variables?: Record<string, string | number>) => string): string {
  const key = `ai.notifications.preferences.types.${code}`;
  const translated = t(key);
  return translated === key ? code : translated;
}

function labelForChannel(code: string, t: (key: string, variables?: Record<string, string | number>) => string): string {
  const key = `ai.notifications.preferences.channels.${code}`;
  const translated = t(key);
  return translated === key ? code : translated;
}

type NotificationPreferencesPageProps = {
  /** Dentro de Ajustes: oculta el encabezado principal duplicado. */
  embedded?: boolean;
};

export function NotificationPreferencesPage({ embedded = false }: NotificationPreferencesPageProps) {
  const { t } = useI18n();
  const npSearch = usePageSearch();
  const npTextFn = useCallback(
    (p: NotificationPreference) =>
      `${p.notification_type} ${p.channel} ${labelForType(p.notification_type, t)} ${labelForChannel(p.channel, t)}`,
    [t],
  );
  const queryClient = useQueryClient();
  const preferencesQuery = useQuery({
    queryKey: queryKeys.notifications.preferences,
    queryFn: getNotificationPreferences,
  });
  const items = preferencesQuery.data?.items ?? [];
  const filteredPrefs = useSearch(items, npTextFn, npSearch);
  const toggleMutation = useMutation({
    mutationFn: async (item: NotificationPreference) =>
      updateNotificationPreference({
        notification_type: item.notification_type,
        channel: item.channel,
        enabled: !item.enabled,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.notifications.preferences });
    },
  });
  const rawError =
    preferencesQuery.error instanceof Error
      ? preferencesQuery.error
      : toggleMutation.error instanceof Error
        ? toggleMutation.error
        : null;
  const error = rawError
    ? formatFetchErrorForUser(rawError, t('ai.notifications.preferences.error.load'))
    : '';

  const body = (
    <>
      {error && <div className="alert alert-error">{error}</div>}

      <div className="card">
        {preferencesQuery.isLoading ? (
          <div className="empty-state">
            <p>{t('ai.notifications.preferences.loading')}</p>
          </div>
        ) : items.length === 0 ? (
          <div className="empty-state">
            <p>{t('ai.notifications.preferences.empty')}</p>
          </div>
        ) : filteredPrefs.length === 0 ? (
          <div className="empty-state">
            <p>{t('ai.notifications.preferences.emptySearch')}</p>
          </div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>{t('ai.notifications.preferences.table.type')}</th>
                  <th>{t('ai.notifications.preferences.table.channel')}</th>
                  <th>{t('ai.notifications.preferences.table.active')}</th>
                </tr>
              </thead>
              <tbody>
                {filteredPrefs.map((item) => (
                  <tr key={`${item.notification_type}-${item.channel}`}>
                    <td className="text-semibold">{labelForType(item.notification_type, t)}</td>
                    <td>
                      <span className="badge badge-neutral">{labelForChannel(item.channel, t)}</span>
                    </td>
                    <td>
                      <label className="toggle" onClick={() => void toggleMutation.mutateAsync(item)}>
                        <input
                          type="checkbox"
                          aria-label={`${t('ai.notifications.preferences.toggleAction')} ${labelForType(
                            item.notification_type,
                            t,
                          )} ${t('ai.notifications.preferences.toggleVia')} ${labelForChannel(item.channel, t)}`}
                          checked={item.enabled}
                          disabled={toggleMutation.isPending}
                          readOnly
                        />
                        <span className="toggle-track" />
                        <span className="toggle-thumb" />
                      </label>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </>
  );

  if (embedded) {
    return <div>{body}</div>;
  }

  return (
    <PageLayout
      title={t('ai.notifications.preferences.pageTitle')}
      lead={t('ai.notifications.preferences.pageLead')}
    >
      {body}
    </PageLayout>
  );
}
