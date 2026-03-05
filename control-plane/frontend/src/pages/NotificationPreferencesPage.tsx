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
    <div className="card">
      <h1>Notification Preferences</h1>
      {error && <p style={{ color: 'crimson' }}>{error}</p>}
      <table style={{ width: '100%' }}>
        <thead>
          <tr>
            <th align="left">Type</th>
            <th align="left">Channel</th>
            <th align="left">Enabled</th>
          </tr>
        </thead>
        <tbody>
          {items.map((item) => (
            <tr key={`${item.notification_type}-${item.channel}`}>
              <td>{item.notification_type}</td>
              <td>{item.channel}</td>
              <td>
                <button className="secondary" onClick={() => void toggle(item)}>
                  {item.enabled ? 'On' : 'Off'}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
