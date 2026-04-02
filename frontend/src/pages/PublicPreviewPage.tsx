import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { PageLayout } from '../components/PageLayout';
import { queryKeys } from '../lib/queryKeys';
import { getPublicTeachers, getTeachersPreviewBootstrap } from '../lib/teachersApi';

export function PublicPreviewPage() {
  const [orgSlug, setOrgSlug] = useState('');
  const [activeSlug, setActiveSlug] = useState('');
  const slugInputId = 'public-preview-org-slug';

  const bootstrapQuery = useQuery({
    queryKey: queryKeys.teachers.previewBootstrap,
    queryFn: getTeachersPreviewBootstrap,
  });
  const teachersQuery = useQuery({
    queryKey: queryKeys.teachers.publicBySlug(activeSlug),
    queryFn: () => getPublicTeachers(activeSlug),
    enabled: activeSlug.trim().length > 0,
  });

  useEffect(() => {
    const slug = bootstrapQuery.data?.slug?.trim();
    if (!slug) return;
    setOrgSlug(slug);
    setActiveSlug(slug);
  }, [bootstrapQuery.data]);

  function handleLoad() {
    if (orgSlug.trim()) {
      setActiveSlug(orgSlug.trim());
    }
  }

  const bootstrapSlugError =
    bootstrapQuery.isSuccess && !bootstrapQuery.data.slug?.trim()
      ? 'No se pudo resolver el slug publico de la organizacion.'
      : '';
  const error = bootstrapSlugError
    || (bootstrapQuery.error instanceof Error ? bootstrapQuery.error.message : '')
    || (teachersQuery.error instanceof Error ? teachersQuery.error.message : '');
  const items = teachersQuery.data?.items ?? [];
  const loading = bootstrapQuery.isLoading || teachersQuery.isFetching;

  return (
    <PageLayout
      title="Vista pública"
      lead="Vista previa de cómo se vería la página pública del módulo teachers"
    >
      {error && <div className="alert alert-error">{error}</div>}

      <div className="card u-mb-md">
        <div className="card-header">
          <h2>Configuración</h2>
        </div>
        <div className="form-row">
          <div className="form-group grow">
            <label htmlFor={slugInputId}>Slug de organización</label>
            <input
              id={slugInputId}
              value={orgSlug}
              onChange={(e) => setOrgSlug(e.target.value)}
              placeholder="slug-organizacion"
            />
          </div>
          <button type="button" className="btn-primary" onClick={handleLoad} disabled={!orgSlug.trim()}>
            Cargar
          </button>
        </div>
      </div>

      <div className="alert alert-warning u-mb-md">
        Esta es una vista previa. La pagina publica real se sirve desde una URL separada.
      </div>

      {loading ? (
        <div className="spinner" />
      ) : items.length === 0 ? (
        <div className="card">
          <div className="empty-state">
            <p>No hay teachers públicos para esta organización</p>
          </div>
        </div>
      ) : (
        <div className="grid-cards-auto">
          {items.map((item) => (
            <div key={item.id} className="card">
              <div className="card-header">
                <h2>{item.headline || item.public_slug || 'Teacher'}</h2>
                {item.is_bookable && <span className="badge badge-success">Reservable</span>}
              </div>
              {item.bio && <p className="text-secondary-lead">{item.bio}</p>}
              {(item.specialties ?? []).length > 0 && (
                <div className="actions-row actions-row--wrap">
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
                <div className="u-mt-sm">
                  <span className="text-secondary">Slug: </span>
                  <span className="mono">{item.public_slug}</span>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </PageLayout>
  );
}
