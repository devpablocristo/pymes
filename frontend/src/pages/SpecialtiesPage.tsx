import { useEffect, useState } from 'react';
import { getSpecialties, createSpecialty, updateSpecialty } from '../lib/professionalsApi';
import type { Specialty } from '../lib/professionalsTypes';

export function SpecialtiesPage() {
  const [items, setItems] = useState<Specialty[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Inline create form
  const [code, setCode] = useState('');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');

  async function load() {
    try {
      setLoading(true);
      const resp = await getSpecialties();
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
    if (!code.trim() || !name.trim()) return;
    try {
      setError('');
      await createSpecialty({ code, name, description });
      setCode('');
      setName('');
      setDescription('');
      await load();
    } catch (err) {
      setError(String(err));
    }
  }

  async function toggleActive(item: Specialty) {
    try {
      setError('');
      await updateSpecialty(item.id, { is_active: !item.is_active });
      await load();
    } catch (err) {
      setError(String(err));
    }
  }

  return (
    <>
      <div className="page-header">
        <h1>Especialidades</h1>
        <p>Catálogo de especialidades profesionales</p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="card" style={{ marginBottom: '1rem' }}>
        <div className="card-header">
          <h2>Crear especialidad</h2>
        </div>
        <div className="form-row" style={{ marginBottom: '0.75rem' }}>
          <div className="form-group grow">
            <label>Codigo</label>
            <input value={code} onChange={(e) => setCode(e.target.value)} placeholder="ej: PSY" />
          </div>
          <div className="form-group grow">
            <label>Nombre</label>
            <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Nombre de la especialidad" />
          </div>
          <div className="form-group grow">
            <label>Descripción</label>
            <input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Descripción breve" />
          </div>
          <button className="btn-primary" onClick={handleCreate}>Crear</button>
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <h2>Listado</h2>
          <span className="badge badge-neutral">{items.length} especialidades</span>
        </div>
        {loading ? (
          <div className="spinner" />
        ) : items.length === 0 ? (
          <div className="empty-state">
            <p>No hay especialidades registradas</p>
          </div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Codigo</th>
                  <th>Nombre</th>
                  <th>Descripción</th>
                  <th>Estado</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id}>
                    <td className="mono">{item.code}</td>
                    <td><strong>{item.name}</strong></td>
                    <td className="text-secondary">{item.description || '-'}</td>
                    <td>
                      <label className="toggle" onClick={() => toggleActive(item)}>
                        <input type="checkbox" checked={item.is_active} readOnly />
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
