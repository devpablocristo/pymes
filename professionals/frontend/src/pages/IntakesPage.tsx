import { useEffect, useState } from 'react';
import { getIntakes, createIntake, submitIntake, updateIntake } from '../lib/api';
import type { Intake } from '../lib/types';

const statusBadge: Record<string, string> = {
  draft: 'badge-neutral',
  submitted: 'badge-warning',
  reviewed: 'badge-success',
};

export function IntakesPage() {
  const [items, setItems] = useState<Intake[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [filterStatus, setFilterStatus] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);

  // Create form
  const [profileId, setProfileId] = useState('');
  const [notes, setNotes] = useState('');

  // Edit form
  const [editNotes, setEditNotes] = useState('');

  async function load() {
    try {
      setLoading(true);
      const resp = await getIntakes();
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

  const filtered = filterStatus ? items.filter((i) => i.status === filterStatus) : items;

  async function handleCreate() {
    if (!profileId.trim()) return;
    try {
      setError('');
      await createIntake({ profile_id: profileId, notes });
      setProfileId('');
      setNotes('');
      setShowForm(false);
      await load();
    } catch (err) {
      setError(String(err));
    }
  }

  async function handleSubmit(id: string) {
    try {
      setError('');
      await submitIntake(id);
      await load();
    } catch (err) {
      setError(String(err));
    }
  }

  async function handleUpdate(id: string) {
    try {
      setError('');
      await updateIntake(id, { notes: editNotes });
      setEditingId(null);
      await load();
    } catch (err) {
      setError(String(err));
    }
  }

  function startEdit(item: Intake) {
    setEditingId(item.id);
    setEditNotes(item.notes);
  }

  return (
    <>
      <div className="page-header">
        <h1>Intakes</h1>
        <p>Gestion de procesos de ingreso</p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="actions-row" style={{ marginBottom: '1rem', gap: '0.75rem' }}>
        <button className="btn-primary" onClick={() => setShowForm(!showForm)}>
          {showForm ? 'Cancelar' : 'Nuevo intake'}
        </button>
        <select value={filterStatus} onChange={(e) => setFilterStatus(e.target.value)}>
          <option value="">Todos los estados</option>
          <option value="draft">Borrador</option>
          <option value="submitted">Enviado</option>
          <option value="reviewed">Revisado</option>
        </select>
      </div>

      {showForm && (
        <div className="card" style={{ marginBottom: '1rem' }}>
          <div className="card-header">
            <h2>Crear intake</h2>
          </div>
          <div className="form-row" style={{ marginBottom: '0.75rem' }}>
            <div className="form-group grow">
              <label>Profile ID</label>
              <input value={profileId} onChange={(e) => setProfileId(e.target.value)} placeholder="ID del profesional" />
            </div>
          </div>
          <div className="form-group" style={{ marginBottom: '0.75rem' }}>
            <label>Notas</label>
            <textarea value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Notas del intake" rows={3} style={{ width: '100%' }} />
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
          <span className="badge badge-neutral">{filtered.length} intakes</span>
        </div>
        {loading ? (
          <div className="spinner" />
        ) : filtered.length === 0 ? (
          <div className="empty-state">
            <p>No hay intakes{filterStatus ? ` con estado "${filterStatus}"` : ''}</p>
          </div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Profesional</th>
                  <th>Estado</th>
                  <th>Creado</th>
                  <th>Notas</th>
                  <th>Acciones</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((item) => (
                  <tr key={item.id}>
                    <td className="mono">{item.profile_id}</td>
                    <td>
                      <span className={`badge ${statusBadge[item.status] ?? 'badge-neutral'}`}>
                        {item.status}
                      </span>
                    </td>
                    <td>{new Date(item.created_at).toLocaleDateString('es-AR')}</td>
                    <td className="text-secondary">{item.notes ? item.notes.slice(0, 60) : '-'}</td>
                    <td>
                      <div className="actions-row">
                        <button className="btn-secondary btn-sm" onClick={() => startEdit(item)}>Editar</button>
                        {item.status === 'draft' && (
                          <button className="btn-primary btn-sm" onClick={() => handleSubmit(item.id)}>Enviar</button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {editingId && (() => {
        const item = items.find((i) => i.id === editingId);
        if (!item) return null;
        return (
          <div className="card" style={{ marginTop: '1rem' }}>
            <div className="card-header">
              <h2>Editar intake</h2>
              <span className={`badge ${statusBadge[item.status] ?? 'badge-neutral'}`}>{item.status}</span>
            </div>
            <div className="form-group" style={{ marginBottom: '0.75rem' }}>
              <label>Notas</label>
              <textarea value={editNotes} onChange={(e) => setEditNotes(e.target.value)} rows={4} style={{ width: '100%' }} />
            </div>
            <div className="actions-row">
              <button className="btn-primary" onClick={() => handleUpdate(editingId)}>Guardar</button>
              <button className="btn-secondary" onClick={() => setEditingId(null)}>Cancelar</button>
            </div>
          </div>
        );
      })()}
    </>
  );
}
