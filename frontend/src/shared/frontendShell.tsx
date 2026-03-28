import { type PropsWithChildren, type ReactNode } from 'react';
import { NavLink, useLocation } from 'react-router-dom';
import { AppShell as ShellSidebar, type AppShellNavItem, type AppShellNavSection } from '@devpablocristo/modules-shell-sidebar';
import '@devpablocristo/modules-shell-sidebar/styles.css';
import { useI18n } from '../lib/i18n';

export type { AppShellNavItem, AppShellNavSection };

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
  const { sentenceCase } = useI18n();

  return (
    <ShellSidebar
      brandTitle={brandTitle}
      brandSubtitle={brandSubtitle}
      sections={sections}
      footerContent={footerContent}
      pathname={location.pathname}
      formatLabel={sentenceCase}
      renderLink={(item, className) => (
        <NavLink
          key={item.to}
          to={item.to}
          end={item.end}
          className={({ isActive: navLinkActive }) => {
            const active = item.isActive
              ? item.isActive(location.pathname, location.search)
              : navLinkActive;
            return `${className}${active ? ' active' : ''}`;
          }}
        >
          {item.icon}
          <span>{sentenceCase(item.label)}</span>
        </NavLink>
      )}
    >
      {children}
    </ShellSidebar>
  );
}
