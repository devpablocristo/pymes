import { useEffect, useState } from 'react';
import { getSessions } from '../lib/api';
import type { Session } from '../lib/types';

export function DashboardPage() {
  const [todaySessions, setTodaySessions] = useState<Session[]>([]);
  const [pendingSessions, setPendingSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const today = new Date().toLocaleDateString('es-AR', {
    weekday: 'long',
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  });

  useEffect(() => {
    (async () => {
      try {
        const [allResp, scheduledResp] = await Promise.all([
          getSessions().catch(() => ({ items: [] })),
          getSessions({ status: 'scheduled' }).catch(() => ({ items: [] })),
        ]);

        const todayStr = new Date().toISOString().slice(0, 10);
        const todayItems = (allResp.items ?? []).filter(
          (s) => s.started_at && s.started_at.startsWith(todayStr),
        );
        setTodaySessions(todayItems);
        setPendingSessions(scheduledResp.items ?? []);
      } catch (err) {
        setError(String(err));
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  return (
    <>
      <div className="page-header">
        <h1>Agenda del dia</h1>
        <p>{today}</p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="stats-grid">
        <div className="stat-card">
          <div className="stat-label">Turnos hoy</div>
          <div className="stat-value">{loading ? '...' : todaySessions.length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Sesiones pendientes</div>
          <div className="stat-value">{loading ? '...' : pendingSessions.length}</div>
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <h2>Turnos de hoy</h2>
          <span className="badge badge-neutral">{todaySessions.length} turnos</span>
        </div>
        {loading ? (
          <div className="spinner" />
        ) : todaySessions.length === 0 ? (
          <div className="empty-state">
            <p>No hay turnos programados para hoy</p>
          </div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Profesional</th>
                  <th>Inicio</th>
                  <th>Estado</th>
                  <th>Resumen</th>
                </tr>
              </thead>
              <tbody>
                {todaySessions.map((session) => (
                  <tr key={session.id}>
                    <td className="mono">{session.profile_id}</td>
                    <td>{session.started_at ? new Date(session.started_at).toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit' }) : '-'}</td>
                    <td>
                      <span className={`badge ${session.status === 'completed' ? 'badge-success' : session.status === 'scheduled' ? 'badge-neutral' : 'badge-warning'}`}>
                        {session.status}
                      </span>
                    </td>
                    <td>{session.summary ? session.summary.slice(0, 60) : '-'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div className="card">
        <div className="card-header">
          <h2>Sesiones pendientes</h2>
          <span className="badge badge-neutral">{pendingSessions.length} pendientes</span>
        </div>
        {loading ? (
          <div className="spinner" />
        ) : pendingSessions.length === 0 ? (
          <div className="empty-state">
            <p>No hay sesiones pendientes</p>
          </div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Profesional</th>
                  <th>Inicio programado</th>
                  <th>Estado</th>
                </tr>
              </thead>
              <tbody>
                {pendingSessions.slice(0, 10).map((session) => (
                  <tr key={session.id}>
                    <td className="mono">{session.profile_id}</td>
                    <td>{session.started_at ? new Date(session.started_at).toLocaleDateString('es-AR') : '-'}</td>
                    <td><span className="badge badge-neutral">{session.status}</span></td>
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
