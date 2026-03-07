import { useEffect, useState } from 'react';
import { getProfessionals, createProfessional, updateProfessional } from '../lib/professionalsApi';
import type { ProfessionalProfile } from '../lib/professionalsTypes';

export function ProfessionalsPage() {
  const [items, setItems] = useState<ProfessionalProfile[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);

  // Create form
  const [partyId, setPartyId] = useState('');
  const [bio, setBio] = useState('');
  const [headline, setHeadline] = useState('');
  const [publicSlug, setPublicSlug] = useState('');

  // Edit form
  const [editBio, setEditBio] = useState('');
  const [editHeadline, setEditHeadline] = useState('');
  const [editSlug, setEditSlug] = useState('');

  async function load() {
    try {
      setLoading(true);
      const resp = await getProfessionals();
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
    if (!partyId.trim()) return;
    try {
      setError('');
      await createProfessional({ party_id: partyId, bio, headline, public_slug: publicSlug });
      setPartyId('');
      setBio('');
      setHeadline('');
      setPublicSlug('');
      setShowForm(false);
      await load();
    } catch (err) {
      setError(String(err));
    }
  }

  async function handleUpdate(id: string) {
    try {
      setError('');
      await updateProfessional(id, { bio: editBio, headline: editHeadline, public_slug: editSlug });
      setEditingId(null);
      await load();
    } catch (err) {
      setError(String(err));
    }
  }

  async function togglePublic(item: ProfessionalProfile) {
    try {
      await updateProfessional(item.id, { is_public: !item.is_public });
      await load();
    } catch (err) {
      setError(String(err));
    }
  }

  async function toggleBookable(item: ProfessionalProfile) {
    try {
      await updateProfessional(item.id, { is_bookable: !item.is_bookable });
      await load();
    } catch (err) {
      setError(String(err));
    }
  }

  function startEdit(item: ProfessionalProfile) {
    setEditingId(item.id);
    setEditBio(item.bio);
    setEditHeadline(item.headline);
    setEditSlug(item.public_slug);
  }

  return (
    <>
      <div className="page-header">
        <h1>Profesionales</h1>
        <p>Gestión de perfiles profesionales</p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="actions-row" style={{ marginBottom: '1rem' }}>
        <button className="btn-primary" onClick={() => setShowForm(!showForm)}>
          {showForm ? 'Cancelar' : 'Nuevo profesional'}
        </button>
      </div>

      {showForm && (
        <div className="card" style={{ marginBottom: '1rem' }}>
          <div className="card-header">
            <h2>Crear profesional</h2>
          </div>
          <div className="form-row" style={{ marginBottom: '0.75rem' }}>
            <div className="form-group grow">
              <label>ID de entidad</label>
              <input value={partyId} onChange={(e) => setPartyId(e.target.value)} placeholder="ID de partido o persona" />
            </div>
            <div className="form-group grow">
              <label>Título profesional</label>
              <input value={headline} onChange={(e) => setHeadline(e.target.value)} placeholder="Título profesional" />
            </div>
          </div>
          <div className="form-row" style={{ marginBottom: '0.75rem' }}>
            <div className="form-group grow">
              <label>Slug público</label>
              <input value={publicSlug} onChange={(e) => setPublicSlug(e.target.value)} placeholder="slug-unico" />
            </div>
          </div>
          <div className="form-group" style={{ marginBottom: '0.75rem' }}>
            <label>Bio</label>
            <textarea value={bio} onChange={(e) => setBio(e.target.value)} placeholder="Descripcion profesional" rows={3} style={{ width: '100%' }} />
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
          <span className="badge badge-neutral">{items.length} profesionales</span>
        </div>
        {loading ? (
          <div className="spinner" />
        ) : items.length === 0 ? (
          <div className="empty-state">
            <p>No hay profesionales registrados</p>
          </div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Nombre / Headline</th>
                  <th>Slug</th>
                  <th>Especialidades</th>
                  <th>Público</th>
                  <th>Reservable</th>
                  <th>Acciones</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id}>
                    <td>
                      <strong>{item.headline || item.party_id}</strong>
                      {item.bio && <div className="text-secondary">{item.bio.slice(0, 80)}</div>}
                    </td>
                    <td className="mono">{item.public_slug || '-'}</td>
                    <td>{(item.specialties ?? []).join(', ') || '-'}</td>
                    <td>
                      <span
                        className={`badge ${item.is_public ? 'badge-success' : 'badge-neutral'}`}
                        style={{ cursor: 'pointer' }}
                        onClick={() => togglePublic(item)}
                      >
                        {item.is_public ? 'Si' : 'No'}
                      </span>
                    </td>
                    <td>
                      <span
                        className={`badge ${item.is_bookable ? 'badge-success' : 'badge-neutral'}`}
                        style={{ cursor: 'pointer' }}
                        onClick={() => toggleBookable(item)}
                      >
                        {item.is_bookable ? 'Si' : 'No'}
                      </span>
                    </td>
                    <td>
                      <button className="btn-secondary btn-sm" onClick={() => startEdit(item)}>Editar</button>
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
              <h2>Editar: {item.headline || item.party_id}</h2>
            </div>
            <div className="form-row" style={{ marginBottom: '0.75rem' }}>
              <div className="form-group grow">
                <label>Título profesional</label>
                <input value={editHeadline} onChange={(e) => setEditHeadline(e.target.value)} />
              </div>
              <div className="form-group grow">
                <label>Slug público</label>
                <input value={editSlug} onChange={(e) => setEditSlug(e.target.value)} />
              </div>
            </div>
            <div className="form-group" style={{ marginBottom: '0.75rem' }}>
              <label>Bio</label>
              <textarea value={editBio} onChange={(e) => setEditBio(e.target.value)} rows={3} style={{ width: '100%' }} />
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
