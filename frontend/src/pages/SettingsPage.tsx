import { UserProfile } from '@clerk/clerk-react';
import { clerkEnabled } from '@pymes/ts-pkg/auth';

export function SettingsPage() {
  return (
    <>
      <div className="page-header">
        <h1>Perfil</h1>
        <p>Gestiona tu cuenta y preferencias</p>
      </div>
      <div className="card">
        {clerkEnabled ? (
          <UserProfile routing="path" path="/settings" />
        ) : (
          <div className="empty-state">
            <p>Clerk deshabilitado. Este entorno usa autenticacion por clave API.</p>
          </div>
        )}
      </div>
    </>
  );
}
