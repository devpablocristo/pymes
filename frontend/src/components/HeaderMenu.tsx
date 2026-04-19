import { useEffect, useRef } from 'react';
import { tenantLink, useTenantSlug } from '../lib/tenantSlug';
import { type HeaderMenuItem } from './HeaderMenuContext';
import './SettingsMenu.css';

export function HeaderMenu({ items = [] }: { items?: HeaderMenuItem[] }) {
  const slug = useTenantSlug();
  const detailsRef = useRef<HTMLDetailsElement | null>(null);
  const resolvedItems: HeaderMenuItem[] = [
    ...items,
    { label: 'Ajustes', href: tenantLink('/settings', slug) },
  ];

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      const details = detailsRef.current;
      if (!details?.open) return;
      details.open = false;
      const summary = details.querySelector('summary');
      if (summary instanceof HTMLElement) summary.focus();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  return (
    <details ref={detailsRef} className="settings-menu">
      <summary className="settings-menu__trigger" aria-label="Abrir menú" title="Menú">
        <span className="settings-menu__avatar" aria-hidden>
          <span className="settings-menu__avatar-head" />
          <span className="settings-menu__avatar-body" />
        </span>
      </summary>
      <div className="settings-menu__panel">
        {resolvedItems.map((item) => (
          <a key={`${item.label}:${item.href}`} className="settings-menu__item" href={item.href}>
            {item.label}
          </a>
        ))}
      </div>
    </details>
  );
}
