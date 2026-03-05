import { UserProfile } from '@clerk/clerk-react';
import { clerkEnabled } from '../lib/auth';

export function SettingsPage() {
  return (
    <>
      <div className="page-header">
        <h1>Profile</h1>
        <p>Gestiona tu cuenta y preferencias</p>
      </div>
      <div className="card">
        {clerkEnabled ? (
          <UserProfile routing="path" path="/settings" />
        ) : (
          <div className="empty-state">
            <p>Clerk deshabilitado. Este entorno usa autenticacion por API key.</p>
          </div>
        )}
      </div>
    </>
  );
}
