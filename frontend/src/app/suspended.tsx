import { Suspense, type ReactNode } from 'react';

export function Suspended({ children }: { children: ReactNode }) {
  return <Suspense fallback={<div className="card"><p>Cargando…</p></div>}>{children}</Suspense>;
}
