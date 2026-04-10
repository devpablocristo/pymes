import { type PropsWithChildren, type ReactNode } from 'react';
import { NavLink, useLocation } from 'react-router-dom';
import {
  PageShellFrame,
  type AppShellNavItem,
  type AppShellNavSection,
} from '@devpablocristo/modules-ui-page-shell';
import '@devpablocristo/modules-ui-page-shell/styles.css';
import { useI18n } from '../lib/i18n';
import { NotificationBadge } from '../components/NotificationBadge';

export type { AppShellNavItem, AppShellNavSection };

export function AppShell({
  children,
  brandTitle,
  brandSubtitle,
  sections,
  footerContent,
  searchPlaceholder,
  skipLinkLabel,
}: PropsWithChildren<{
  brandTitle: string;
  brandSubtitle: string;
  sections: AppShellNavSection[];
  footerContent?: ReactNode;
  searchPlaceholder?: string;
  skipLinkLabel?: string;
}>) {
  const location = useLocation();
  const { sentenceCase } = useI18n();

  return (
    <PageShellFrame
      brandTitle={brandTitle}
      brandSubtitle={brandSubtitle}
      sections={sections}
      footerContent={footerContent}
      pathname={location.pathname}
      formatLabel={sentenceCase}
      searchPlaceholder={searchPlaceholder}
      skipLinkLabel={skipLinkLabel}
      renderLink={(item, className) => (
        <NavLink
          key={item.to}
          to={item.to}
          end={item.end}
          className={({ isActive: navLinkActive }) => {
            const active = item.isActive ? item.isActive(location.pathname, location.search) : navLinkActive;
            return `${className}${active ? ' active' : ''}`;
          }}
        >
          {item.icon}
          <span>{sentenceCase(item.label)}</span>
          {item.to === '/notifications' && <NotificationBadge />}
        </NavLink>
      )}
    >
      {children}
    </PageShellFrame>
  );
}
