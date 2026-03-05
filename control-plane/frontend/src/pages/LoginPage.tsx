import { Link } from 'react-router-dom';
import { SignIn } from '@clerk/clerk-react';
import { clerkEnabled } from '../lib/auth';

export function LoginPage() {
  if (clerkEnabled) {
    return (
      <div className="auth-layout">
        <SignIn routing="path" path="/login" signUpUrl="/signup" />
      </div>
    );
  }
  return (
    <div className="auth-layout">
      <div className="auth-card">
        <h1>Login local</h1>
        <p>Clerk deshabilitado. Usa API key para consumir la API desde el frontend.</p>
        <Link to="/">Ir al dashboard</Link>
      </div>
    </div>
  );
}
