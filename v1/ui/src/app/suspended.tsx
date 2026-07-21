import { Suspense, type ReactNode } from 'react';
import { PageLayout } from '../components/PageLayout';

export function Suspended({ children }: { children: ReactNode }) {
  return (
    <Suspense
      fallback={
        <PageLayout title="Cargando" lead="Preparando la vista solicitada.">
          <div className="card">
            <p>Cargando…</p>
          </div>
        </PageLayout>
      }
    >
      {children}
    </Suspense>
  );
}
