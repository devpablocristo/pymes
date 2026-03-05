import type { PropsWithChildren } from 'react';
import { NavLink } from 'react-router-dom';
import { UserButton } from '@clerk/clerk-react';
import { clerkEnabled } from '../lib/auth';

const navItems = [
  { to: '/', label: 'Dashboard' },
  { to: '/admin', label: 'Admin' },
  { to: '/billing', label: 'Billing' },
  { to: '/settings/keys', label: 'API Keys' },
  { to: '/settings/notifications', label: 'Notifications' },
  { to: '/settings', label: 'Profile' },
];

export function Shell({ children }: PropsWithChildren) {
  return (
    <div>
      <header className="container" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <h2 style={{ marginBottom: '0.25rem' }}>Pymes SaaS</h2>
          <small>Base Transversal</small>
        </div>
        {clerkEnabled ? <UserButton /> : <span>Auth: API Key local</span>}
      </header>
      <nav className="container" style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap' }}>
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            className={({ isActive }) => (isActive ? 'active' : '')}
          >
            {item.label}
          </NavLink>
        ))}
      </nav>
      <main className="container">{children}</main>
    </div>
  );
}
