import { useEffect, useState } from 'react';
import { getSessions, createSession, completeSession, addSessionNote } from '../lib/api';
import type { Session } from '../lib/types';

const statusBadge: Record<string, string> = {
  scheduled: 'badge-neutral',
  in_progress: 'badge-warning',
  completed: 'badge-success',
  cancelled: 'badge-danger',
};

export function SessionsPage() {
  const [items, setItems] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [noteSessionId, setNoteSessionId] = useState<string | null>(null);

  // Create form
  const [profileId, setProfileId] = useState('');
  const [startedAt, setStartedAt] = useState('');
  const [summary, setSummary] = useState('');

  // Note form
  const [noteContent, setNoteContent] = useState('');
  const [noteAuthor, setNoteAuthor] = useState('');

  async function load() {
    try {
      setLoading(true);
      const resp = await getSessions();
      setItems(resp.items ?? []);
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function handleCreate() {
    if (!profileId.trim() || !startedAt.trim()) return;
    try {
      setError('');
      await createSession({ profile_id: profileId, started_at: startedAt, summary: summary || undefined });
      setProfileId('');
      setStartedAt('');
      setSummary('');
      setShowForm(false);
      await load();
    } catch (err) {
      setError(String(err));
    }
  }

  async function handleComplete(id: string) {
    try {
      setError('');
      await completeSession(id);
      await load();
    } catch (err) {
      setError(String(err));
    }
  }

  async function handleAddNote() {
    if (!noteSessionId || !noteContent.trim()) return;
    try {
      setError('');
      await addSessionNote(noteSessionId, { content: noteContent, author: noteAuthor || 'admin' });
      setNoteContent('');
      setNoteAuthor('');
      setNoteSessionId(null);
      await load();
    } catch (err) {
      setError(String(err));
    }
  }

  return (
    <>
      <div className="page-header">
        <h1>Sesiones</h1>
        <p>Gestion de sesiones profesionales</p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="actions-row" style={{ marginBottom: '1rem' }}>
        <button className="btn-primary" onClick={() => setShowForm(!showForm)}>
          {showForm ? 'Cancelar' : 'Nueva sesion'}
        </button>
      </div>

      {showForm && (
        <div className="card" style={{ marginBottom: '1rem' }}>
          <div className="card-header">
            <h2>Crear sesion</h2>
          </div>
          <div className="form-row" style={{ marginBottom: '0.75rem' }}>
            <div className="form-group grow">
              <label>Profile ID</label>
              <input value={profileId} onChange={(e) => setProfileId(e.target.value)} placeholder="ID del profesional" />
            </div>
            <div className="form-group grow">
              <label>Fecha/hora inicio</label>
              <input type="datetime-local" value={startedAt} onChange={(e) => setStartedAt(e.target.value)} />
            </div>
          </div>
          <div className="form-group" style={{ marginBottom: '0.75rem' }}>
            <label>Resumen (opcional)</label>
            <textarea value={summary} onChange={(e) => setSummary(e.target.value)} placeholder="Resumen de la sesion" rows={2} style={{ width: '100%' }} />
          </div>
          <div className="actions-row">
            <button className="btn-primary" onClick={handleCreate}>Crear</button>
            <button className="btn-secondary" onClick={() => setShowForm(false)}>Cancelar</button>
          </div>
        </div>
      )}

      <div className="card">
        <div className="card-header">
          <h2>Listado</h2>
          <span className="badge badge-neutral">{items.length} sesiones</span>
        </div>
        {loading ? (
          <div className="spinner" />
        ) : items.length === 0 ? (
          <div className="empty-state">
            <p>No hay sesiones registradas</p>
          </div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Profesional</th>
                  <th>Estado</th>
                  <th>Inicio</th>
                  <th>Fin</th>
                  <th>Resumen</th>
                  <th>Acciones</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id}>
                    <td className="mono">{item.profile_id}</td>
                    <td>
                      <span className={`badge ${statusBadge[item.status] ?? 'badge-neutral'}`}>
                        {item.status}
                      </span>
                    </td>
                    <td>{item.started_at ? new Date(item.started_at).toLocaleString('es-AR') : '-'}</td>
                    <td>{item.ended_at ? new Date(item.ended_at).toLocaleString('es-AR') : '-'}</td>
                    <td className="text-secondary">{item.summary ? item.summary.slice(0, 50) : '-'}</td>
                    <td>
                      <div className="actions-row">
                        {(item.status === 'scheduled' || item.status === 'in_progress') && (
                          <button className="btn-success btn-sm" onClick={() => handleComplete(item.id)}>Completar</button>
                        )}
                        <button className="btn-secondary btn-sm" onClick={() => setNoteSessionId(item.id)}>Nota</button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {noteSessionId && (
        <div className="card" style={{ marginTop: '1rem' }}>
          <div className="card-header">
            <h2>Agregar nota</h2>
          </div>
          <div className="form-row" style={{ marginBottom: '0.75rem' }}>
            <div className="form-group grow">
              <label>Autor</label>
              <input value={noteAuthor} onChange={(e) => setNoteAuthor(e.target.value)} placeholder="Nombre del autor" />
            </div>
          </div>
          <div className="form-group" style={{ marginBottom: '0.75rem' }}>
            <label>Contenido</label>
            <textarea value={noteContent} onChange={(e) => setNoteContent(e.target.value)} placeholder="Nota de la sesion" rows={3} style={{ width: '100%' }} />
          </div>
          <div className="actions-row">
            <button className="btn-primary" onClick={handleAddNote}>Agregar</button>
            <button className="btn-secondary" onClick={() => setNoteSessionId(null)}>Cancelar</button>
          </div>
        </div>
      )}
    </>
  );
}
