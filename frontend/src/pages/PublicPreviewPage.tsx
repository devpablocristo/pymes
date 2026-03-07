import { useEffect, useState } from 'react';
import { getPublicPreviewBootstrap, getPublicProfessionals } from '../lib/professionalsApi';
import type { ProfessionalProfile } from '../lib/professionalsTypes';

export function PublicPreviewPage() {
  const [items, setItems] = useState<ProfessionalProfile[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [orgSlug, setOrgSlug] = useState('');

  async function load(slug: string) {
    try {
      setLoading(true);
      setError('');
      const resp = await getPublicProfessionals(slug);
      setItems(resp.items ?? []);
    } catch (err) {
      setError(String(err));
      setItems([]);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void (async () => {
      try {
        const bootstrap = await getPublicPreviewBootstrap();
        const slug = bootstrap.slug?.trim();
        if (!slug) {
          setError('No se pudo resolver el slug publico de la organizacion.');
          setLoading(false);
          return;
        }
        setOrgSlug(slug);
        await load(slug);
      } catch (err) {
        setError(String(err));
        setLoading(false);
      }
    })();
  }, []);

  function handleLoad() {
    if (orgSlug.trim()) {
      load(orgSlug.trim());
    }
  }

  return (
    <>
      <div className="page-header">
        <h1>Vista publica</h1>
        <p>Preview de como se veria la pagina publica de profesionales</p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="card" style={{ marginBottom: '1rem' }}>
        <div className="card-header">
          <h2>Configuracion</h2>
        </div>
        <div className="form-row">
          <div className="form-group grow">
            <label>Slug de organizacion</label>
            <input value={orgSlug} onChange={(e) => setOrgSlug(e.target.value)} placeholder="slug-org" />
          </div>
          <button className="btn-primary" onClick={handleLoad}>Cargar</button>
        </div>
      </div>

      <div className="alert alert-warning" style={{ marginBottom: '1rem' }}>
        Esta es una vista previa. La pagina publica real se sirve desde una URL separada.
      </div>

      {loading ? (
        <div className="spinner" />
      ) : items.length === 0 ? (
        <div className="card">
          <div className="empty-state">
            <p>No hay profesionales publicos para esta organizacion</p>
          </div>
        </div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))', gap: '1rem' }}>
          {items.map((item) => (
            <div key={item.id} className="card">
              <div className="card-header">
                <h2>{item.headline || item.public_slug || 'Profesional'}</h2>
                {item.is_bookable && <span className="badge badge-success">Reservable</span>}
              </div>
              {item.bio && <p style={{ color: 'var(--color-text-secondary)', fontSize: '0.87rem', marginBottom: '0.75rem' }}>{item.bio}</p>}
              {(item.specialties ?? []).length > 0 && (
                <div className="actions-row" style={{ flexWrap: 'wrap' }}>
                  {item.specialties.map((spec) => (
                    <span
                      key={typeof spec === 'string' ? spec : spec.id || spec.code || spec.name}
                      className="badge badge-neutral"
                    >
                      {typeof spec === 'string' ? spec : spec.name}
                    </span>
                  ))}
                </div>
              )}
              {item.public_slug && (
                <div style={{ marginTop: '0.75rem' }}>
                  <span className="text-secondary">Slug: </span>
                  <span className="mono">{item.public_slug}</span>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </>
  );
}
