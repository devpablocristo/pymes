import type { PropsWithChildren } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '@clerk/clerk-react';
import { clerkEnabled } from '../lib/auth';

export function ProtectedRoute({ children }: PropsWithChildren) {
  if (!clerkEnabled) {
    return <>{children}</>;
  }

  const { isLoaded, isSignedIn } = useAuth();
  const location = useLocation();
  if (!isLoaded) {
    return (
      <div className="app-layout">
        <div className="main-content">
          <div className="spinner" />
        </div>
      </div>
    );
  }
  if (!isSignedIn) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }
  return <>{children}</>;
}
