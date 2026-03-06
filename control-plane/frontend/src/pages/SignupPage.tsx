import { Link } from 'react-router-dom';
import { SignUp } from '@clerk/clerk-react';
import { clerkEnabled } from '../lib/auth';

export function SignupPage() {
  if (clerkEnabled) {
    return (
      <div className="auth-layout">
        <SignUp routing="path" path="/signup" signInUrl="/login" />
      </div>
    );
  }
  return (
    <div className="auth-layout">
      <div className="auth-card">
        <h1>Registro local</h1>
        <p>Clerk deshabilitado en este ambiente.</p>
        <Link to="/">Ir al panel</Link>
      </div>
    </div>
  );
}
