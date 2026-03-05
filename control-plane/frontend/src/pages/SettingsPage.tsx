import { UserProfile } from '@clerk/clerk-react';
import { clerkEnabled } from '../lib/auth';

export function SettingsPage() {
  return (
    <div className="card">
      <h1>Profile</h1>
      {clerkEnabled ? (
        <UserProfile routing="path" path="/settings" />
      ) : (
        <p>Clerk deshabilitado. Este entorno usa autenticación por API key.</p>
      )}
    </div>
  );
}
