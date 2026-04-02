import { useCallback } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { PageLayout } from '../components/PageLayout';
import { usePageSearch } from '../components/PageSearch';
import { useSearch } from '@devpablocristo/modules-search';
import { getNotificationPreferences, updateNotificationPreference } from '../lib/api';
import { queryKeys } from '../lib/queryKeys';
import type { NotificationPreference } from '../lib/types';

const NOTIFICATION_TYPE_LABELS: Record<string, string> = {
  welcome: 'Bienvenida',
  plan_upgraded: 'Cambio de plan',
  payment_failed: 'Fallo de pago',
  subscription_canceled: 'Suscripción cancelada',
};

const CHANNEL_LABELS: Record<string, string> = {
  email: 'Correo',
};

function labelForType(code: string): string {
  return NOTIFICATION_TYPE_LABELS[code] ?? code;
}

function labelForChannel(code: string): string {
  return CHANNEL_LABELS[code] ?? code;
}

type NotificationPreferencesPageProps = {
  /** Dentro de Ajustes: oculta el encabezado principal duplicado. */
  embedded?: boolean;
};

export function NotificationPreferencesPage({ embedded = false }: NotificationPreferencesPageProps) {
  const npSearch = usePageSearch();
  const npTextFn = useCallback((p: NotificationPreference) => `${p.notification_type} ${p.channel}`, []);
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
  const error = preferencesQuery.error instanceof Error
    ? preferencesQuery.error.message
    : toggleMutation.error instanceof Error
      ? toggleMutation.error.message
      : '';

  const body = (
    <>
      {error && <div className="alert alert-error">{error}</div>}

      <div className="card">
        {preferencesQuery.isLoading ? (
          <div className="empty-state">
            <p>Cargando…</p>
          </div>
        ) : items.length === 0 ? (
          <div className="empty-state">
            <p>No hay tipos de notificación disponibles. Actualizá el backend o contactá soporte.</p>
          </div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Tipo</th>
                  <th>Canal</th>
                  <th>Activo</th>
                </tr>
              </thead>
              <tbody>
                {filteredPrefs.map((item) => (
                  <tr key={`${item.notification_type}-${item.channel}`}>
                    <td className="text-semibold">{labelForType(item.notification_type)}</td>
                    <td>
                      <span className="badge badge-neutral">{labelForChannel(item.channel)}</span>
                    </td>
                    <td>
                      <label className="toggle" onClick={() => void toggleMutation.mutateAsync(item)}>
                        <input
                          type="checkbox"
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
    <PageLayout title="Preferencias de notificación" lead="Elegí qué avisos recibís por canal.">
      {body}
    </PageLayout>
  );
}
