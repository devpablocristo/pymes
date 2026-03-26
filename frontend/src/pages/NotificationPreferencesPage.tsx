import { useEffect, useState } from 'react';
import { getNotificationPreferences, updateNotificationPreference } from '../lib/api';
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
  const [items, setItems] = useState<NotificationPreference[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  async function load(): Promise<void> {
    setLoading(true);
    try {
      const response = await getNotificationPreferences();
      setItems(response.items);
      setError('');
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function toggle(item: NotificationPreference): Promise<void> {
    try {
      await updateNotificationPreference({
        notification_type: item.notification_type,
        channel: item.channel,
        enabled: !item.enabled,
      });
      await load();
    } catch (err) {
      setError(String(err));
    }
  }

  return (
    <>
      {!embedded ? (
        <div className="page-header">
          <h1>Notificaciones</h1>
          <p>Configura como y donde recibis alertas</p>
        </div>
      ) : (
        <div className="card" style={{ marginBottom: 'var(--space-4)' }}>
          <div className="card-header">
            <h2>Preferencias por correo</h2>
          </div>
          <p className="text-secondary" style={{ margin: 0, fontSize: '0.88rem' }}>
            Tipos de aviso que podemos enviar por email según tu organización.
          </p>
        </div>
      )}

      {error && <div className="alert alert-error">{error}</div>}

      <div className="card">
        <div className="card-header">
          <h2>Preferencias</h2>
          <span className="badge badge-neutral">{items.length} reglas</span>
        </div>
        {loading ? (
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
                {items.map((item) => (
                  <tr key={`${item.notification_type}-${item.channel}`}>
                    <td className="text-semibold">{labelForType(item.notification_type)}</td>
                    <td>
                      <span className="badge badge-neutral">{labelForChannel(item.channel)}</span>
                    </td>
                    <td>
                      <label className="toggle" onClick={() => void toggle(item)}>
                        <input type="checkbox" checked={item.enabled} readOnly />
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
}
