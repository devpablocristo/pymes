import { tenantLink, useTenantSlug } from '../lib/tenantSlug';
import { type HeaderMenuItem } from './HeaderMenuContext';
import './SettingsMenu.css';

export function HeaderMenu({ items = [] }: { items?: HeaderMenuItem[] }) {
  const slug = useTenantSlug();
  const resolvedItems: HeaderMenuItem[] = [
    ...items,
    { label: 'Ajustes', href: tenantLink('/settings', slug) },
  ];

  return (
    <details className="settings-menu">
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
