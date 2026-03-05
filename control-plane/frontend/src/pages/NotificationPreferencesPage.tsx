import { useEffect, useState } from 'react';
import { getNotificationPreferences, updateNotificationPreference } from '../lib/api';
import type { NotificationPreference } from '../lib/types';

export function NotificationPreferencesPage() {
  const [items, setItems] = useState<NotificationPreference[]>([]);
  const [error, setError] = useState('');

  async function load(): Promise<void> {
    try {
      const response = await getNotificationPreferences();
      setItems(response.items);
      setError('');
    } catch (err) {
      setError(String(err));
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
      <div className="page-header">
        <h1>Notificaciones</h1>
        <p>Configura como y donde recibis alertas</p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="card">
        <div className="card-header">
          <h2>Preferencias</h2>
          <span className="badge badge-neutral">{items.length} reglas</span>
        </div>
        {items.length === 0 ? (
          <div className="empty-state">
            <p>Sin preferencias configuradas</p>
          </div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Tipo</th>
                  <th>Canal</th>
                  <th>Estado</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={`${item.notification_type}-${item.channel}`}>
                    <td style={{ fontWeight: 500 }}>{item.notification_type}</td>
                    <td>
                      <span className="badge badge-neutral">{item.channel}</span>
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
