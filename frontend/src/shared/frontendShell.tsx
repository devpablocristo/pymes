import { useEffect, type PropsWithChildren, type ReactNode } from 'react';
import { NavLink, useLocation } from 'react-router-dom';
import { useI18n } from '../lib/i18n';

export type AppShellNavItem = {
  to: string;
  label: string;
  icon: ReactNode;
  end?: boolean;
  /** Si se define, reemplaza el criterio por defecto de NavLink (p. ej. Ajustes vs Notificaciones en /settings). */
  isActive?: (pathname: string, search: string) => boolean;
};

export type AppShellNavSection = {
  label: string;
  items: AppShellNavItem[];
};

export function AppShell({
  children,
  brandTitle,
  brandSubtitle,
  sections,
  footerContent,
}: PropsWithChildren<{
  brandTitle: string;
  brandSubtitle: string;
  sections: AppShellNavSection[];
  footerContent?: ReactNode;
}>) {
  const location = useLocation();
  const { t, sentenceCase } = useI18n();

  useEffect(() => {
    const main = document.querySelector<HTMLElement>('.main-content');
    main?.scrollTo({ top: 0, left: 0, behavior: 'auto' });
  }, [location.pathname]);

  return (
    <div className="app-layout">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <h1>{brandTitle}</h1>
          <small>{sentenceCase(brandSubtitle)}</small>
        </div>

        <nav className="sidebar-nav">
          {sections.map((section) => (
            <NavSection key={section.label} label={section.label} items={section.items} />
          ))}
        </nav>

        <div className="sidebar-footer">
          {footerContent ?? null}
        </div>
      </aside>

      <main className="main-content">{children}</main>
    </div>
  );
}

function NavSection({ label, items }: AppShellNavSection) {
  const { sentenceCase } = useI18n();
  const location = useLocation();

  return (
    <>
      <div className="sidebar-section-label">{sentenceCase(label)}</div>
      {items.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          end={item.end}
          className={({ isActive: navLinkActive }) => {
            const active = item.isActive
              ? item.isActive(location.pathname, location.search)
              : navLinkActive;
            return `sidebar-link${active ? ' active' : ''}`;
          }}
        >
          {item.icon}
          <span>{sentenceCase(item.label)}</span>
        </NavLink>
      ))}
    </>
  );
}
