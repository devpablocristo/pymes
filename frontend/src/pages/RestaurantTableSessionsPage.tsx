import { FormEvent, useCallback, useEffect, useState } from 'react';
import { usePageSearch } from '../components/PageSearch';
import { useSearch } from '@devpablocristo/modules-search';
import {
  closeRestaurantTableSession,
  getRestaurantDiningTables,
  getRestaurantTableSessions,
  openRestaurantTableSession,
} from '../lib/restaurantsApi';
import type { RestaurantDiningTable, RestaurantTableSession } from '../lib/restaurantTypes';

function formatSessionDate(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
}

export function RestaurantTableSessionsPage() {
  const [sessions, setSessions] = useState<RestaurantTableSession[]>([]);
  const sessSearch = usePageSearch();
  const sessTextFn = useCallback((s: RestaurantTableSession) => `${s.table_code ?? ''} ${s.area_name ?? ''} ${s.party_label ?? ''}`, []);
  const filteredSessions = useSearch(sessions, sessTextFn, sessSearch);
  const [tables, setTables] = useState<RestaurantDiningTable[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const [tableId, setTableId] = useState('');
  const [guestCount, setGuestCount] = useState('2');
  const [partyLabel, setPartyLabel] = useState('');
  const [notes, setNotes] = useState('');

  const refresh = useCallback(async () => {
    setError('');
    const [sRes, tRes] = await Promise.all([getRestaurantTableSessions(true), getRestaurantDiningTables()]);
    setSessions(sRes.items ?? []);
    setTables(tRes.items ?? []);
  }, []);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      setLoading(true);
      try {
        await refresh();
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'No se pudo cargar el piso.');
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [refresh]);

  async function handleOpen(e: FormEvent) {
    e.preventDefault();
    if (!tableId) {
      return;
    }
    setBusy(true);
    setError('');
    try {
      const n = Math.max(1, Math.min(99, Number.parseInt(guestCount, 10) || 1));
      await openRestaurantTableSession({
        table_id: tableId,
        guest_count: n,
        party_label: partyLabel,
        notes,
      });
      setPartyLabel('');
      setNotes('');
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No se pudo abrir la mesa.');
    } finally {
      setBusy(false);
    }
  }

  async function handleClose(id: string) {
    setBusy(true);
    setError('');
    try {
      await closeRestaurantTableSession(id);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No se pudo cerrar la sesión.');
    } finally {
      setBusy(false);
    }
  }

  const tableOptions = tables.filter((t) => t.status === 'available' || t.status === 'reserved');

  return (
    <>
      <div className="page-header">
        <h1>Sesiones de mesa</h1>
        <p className="text-secondary">
          Apertura y cierre de cuenta en el salón. Ventas y cobros siguen en el módulo comercial del core.
        </p>
      </div>

      {error ? <div className="alert alert-error">{error}</div> : null}

      <section className="card">
        <div className="card-header">
          <h2>Abrir mesa</h2>
        </div>
        <form onSubmit={handleOpen} className="crud-form">
          <div className="crud-form-grid">
            <div className="form-group">
              <label htmlFor="rest-session-table">Mesa</label>
              <select
                id="rest-session-table"
                value={tableId}
                onChange={(ev) => setTableId(ev.target.value)}
                required
                disabled={busy || loading}
              >
                <option value="">Elegir…</option>
                {tableOptions.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.code}
                    {t.label ? ` · ${t.label}` : ''} ({t.status})
                  </option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label htmlFor="rest-session-guests">Comensales</label>
              <input
                id="rest-session-guests"
                type="number"
                min={1}
                max={99}
                value={guestCount}
                onChange={(ev) => setGuestCount(ev.target.value)}
                disabled={busy}
              />
            </div>
            <div className="form-group">
              <label htmlFor="rest-session-party">Reserva / nombre</label>
              <input
                id="rest-session-party"
                value={partyLabel}
                onChange={(ev) => setPartyLabel(ev.target.value)}
                placeholder="Opcional"
                disabled={busy}
              />
            </div>
            <div className="form-group full-width">
              <label htmlFor="rest-session-notes">Notas</label>
              <textarea id="rest-session-notes" value={notes} onChange={(ev) => setNotes(ev.target.value)} rows={2} disabled={busy} />
            </div>
          </div>
          <div className="actions-row">
            <button type="submit" className="btn-primary" disabled={busy || loading || !tableId}>
              {busy ? 'Guardando…' : 'Abrir mesa'}
            </button>
          </div>
        </form>
      </section>

      <section className="card">
        <div className="card-header">
          <h2>Mesas abiertas</h2>
          <button type="button" className="btn-secondary" onClick={() => void refresh()} disabled={busy || loading}>
            Actualizar
          </button>
        </div>
        {loading ? (
          <p className="text-secondary">Cargando…</p>
        ) : sessions.length === 0 ? (
          <p className="text-secondary">No hay mesas abiertas.</p>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Mesa</th>
                  <th>Zona</th>
                  <th>Comensales</th>
                  <th>Etiqueta</th>
                  <th>Abierta</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {filteredSessions.map((s) => (
                  <tr key={s.id}>
                    <td>
                      <strong>{s.table_code ?? s.table_id.slice(0, 8)}</strong>
                    </td>
                    <td>{s.area_name ?? '—'}</td>
                    <td>{s.guest_count}</td>
                    <td>{s.party_label || '—'}</td>
                    <td>{formatSessionDate(s.opened_at)}</td>
                    <td>
                      <button type="button" className="btn-secondary btn-sm" disabled={busy} onClick={() => void handleClose(s.id)}>
                        Cerrar cuenta
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </>
  );
}
