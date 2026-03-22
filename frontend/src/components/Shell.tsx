import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { AppShell, type AppShellNavItem, type AppShellNavSection } from '../shared/frontendShell';
import { getSession } from '../lib/api';
import { moduleGroups, moduleList } from '../lib/moduleCatalog';
import { useI18n } from '../lib/i18n';
import type { ProductRole } from '../lib/types';
import { getVisibleModuleIds } from '../lib/profileFilters';
import { getTenantProfile } from '../lib/tenantProfile';
import { vocab } from '../lib/vocabulary';
import { getTheme, toggleTheme } from '../lib/theme';
function Glyph({ label }: { label: string }) {
  return <span className="sidebar-token">{label}</span>;
}

const dashboardIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <rect x="3" y="3" width="7" height="7" rx="1" />
    <rect x="14" y="3" width="7" height="7" rx="1" />
    <rect x="3" y="14" width="7" height="7" rx="1" />
    <rect x="14" y="14" width="7" height="7" rx="1" />
  </svg>
);

const adminIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z" />
    <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1Z" />
  </svg>
);

const teachersIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
    <circle cx="9" cy="7" r="4" />
    <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
    <path d="M16 3.13a4 4 0 0 1 0 7.75" />
  </svg>
);

const specialtiesIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <path d="M22 12h-4l-3 9L9 3l-3 9H2" />
  </svg>
);

const documentIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
    <polyline points="14 2 14 8 20 8" />
    <line x1="16" y1="13" x2="8" y2="13" />
    <line x1="16" y1="17" x2="8" y2="17" />
    <polyline points="10 9 9 9 8 9" />
  </svg>
);

const clockIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="10" />
    <polyline points="12 6 12 12 16 14" />
  </svg>
);

const globeIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="10" />
    <line x1="2" y1="12" x2="22" y2="12" />
    <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
  </svg>
);

const carIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <path d="M5 17h14" />
    <path d="M6 17l-1 3" />
    <path d="M18 17l1 3" />
    <path d="M5 17V9l2-4h10l2 4v8" />
    <circle cx="7.5" cy="17.5" r="1.5" />
    <circle cx="16.5" cy="17.5" r="1.5" />
  </svg>
);

const wrenchIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <path d="M14.7 6.3a4 4 0 1 0-5.4 5.9L3 18.5V21h2.5l6.3-6.3a4 4 0 0 0 5.9-5.4L21 6l-3-3-3.3 3.3z" />
  </svg>
);

const keyIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4" />
  </svg>
);

const bellIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
    <path d="M13.73 21a2 2 0 0 1-3.46 0" />
  </svg>
);

const profileIcon = (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
    <circle cx="12" cy="7" r="4" />
  </svg>
);

export function Shell({ children }: { children: ReactNode }) {
  const { t, localizeUiText, sentenceCase } = useI18n();
  const [theme, setThemeState] = useState(getTheme);
  const [productRole, setProductRole] = useState<ProductRole | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const session = await getSession();
        if (!cancelled) {
          setProductRole(session.auth.product_role);
        }
      } catch {
        if (!cancelled) {
          setProductRole(null);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const mainNav = useMemo<AppShellNavItem[]>(() => [
    { to: '/', label: t('shell.nav.dashboard'), end: true, icon: dashboardIcon },
  ], [t]);

  const professionalsNav = useMemo<AppShellNavItem[]>(() => [
    { to: '/professionals/teachers', label: t('shell.nav.teachers'), icon: teachersIcon },
    { to: '/professionals/teachers/specialties', label: t('shell.nav.teachersSpecialties'), icon: specialtiesIcon },
    { to: '/professionals/teachers/intakes', label: t('shell.nav.teachersIntakes'), icon: documentIcon },
    { to: '/professionals/teachers/sessions', label: t('shell.nav.teachersSessions'), icon: clockIcon },
    { to: '/professionals/teachers/public', label: t('shell.nav.teachersPublic'), icon: globeIcon },
  ], [t]);

  const workshopsNav = useMemo<AppShellNavItem[]>(() => [
    { to: '/workshops/auto-repair/vehicles', label: t('shell.nav.autoRepairVehicles'), icon: carIcon },
    { to: '/workshops/auto-repair/services', label: t('shell.nav.autoRepairServices'), icon: wrenchIcon },
    { to: '/workshops/auto-repair/orders', label: t('shell.nav.autoRepairOrders'), icon: documentIcon },
  ], [t]);

  const beautyIcon = (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 3v18" />
      <path d="M8 7c0-2 1.5-4 4-4s4 2 4 4c0 3-4 5-4 9" />
      <path d="M16 7c0-2-1.5-4-4-4S8 5 8 7c0 3 4 5 4 9" />
    </svg>
  );

  const beautyNav = useMemo<AppShellNavItem[]>(() => [
    { to: '/beauty/salon/staff', label: t('shell.nav.beautyStaff'), icon: teachersIcon },
    { to: '/beauty/salon/services', label: t('shell.nav.beautyServices'), icon: beautyIcon },
  ], [t]);

  const settingsNav = useMemo<AppShellNavItem[]>(() => {
    const items: AppShellNavItem[] = [];
    if (productRole === 'admin') {
      items.push({ to: '/admin', label: t('shell.nav.admin'), icon: adminIcon });
    }
    items.push(
      { to: '/settings/keys', label: t('shell.nav.apiKeys'), icon: keyIcon },
      { to: '/settings/notifications', label: t('shell.nav.notifications'), icon: bellIcon },
      { to: '/settings', label: t('shell.nav.profile'), end: true, icon: profileIcon },
    );
    return items;
  }, [productRole, t]);

  const sections = useMemo(() => {
    const visibleIds = getVisibleModuleIds();
    const profile = getTenantProfile();
    const vertical = profile?.vertical ?? 'none';

    const moduleNav = moduleGroups.map<AppShellNavSection>((group) => ({
      label: localizeUiText(group.label),
      items: moduleList
        .filter((module) => module.group === group.id && visibleIds.has(module.id))
        .sort((left, right) => localizeUiText(vocab(left.navLabel)).localeCompare(localizeUiText(vocab(right.navLabel))))
        .map((module) => ({
          to: `/modules/${module.id}`,
          label: localizeUiText(vocab(module.navLabel)),
          icon: <Glyph label={module.icon} />,
        })),
    })).filter((section) => section.items.length > 0);

    const result: AppShellNavSection[] = [
      { label: sentenceCase(t('shell.sections.base')), items: mainNav },
    ];
    if (vertical === 'professionals') {
      result.push({ label: sentenceCase(t('shell.sections.professionals')), items: professionalsNav });
    }
    if (vertical === 'workshops') {
      result.push({ label: sentenceCase(t('shell.sections.workshops')), items: workshopsNav });
    }
    if (vertical === 'beauty') {
      result.push({ label: sentenceCase(t('shell.sections.beauty')), items: beautyNav });
    }
    result.push(...moduleNav);
    result.push({ label: sentenceCase(t('shell.sections.settings')), items: settingsNav });
    return result;
  }, [beautyNav, localizeUiText, mainNav, professionalsNav, sentenceCase, settingsNav, t, workshopsNav]);

  function handleToggleTheme() {
    const next = toggleTheme();
    setThemeState(next);
  }

  const themeToggle = (
    <button
      type="button"
      className="theme-toggle"
      onClick={handleToggleTheme}
      title={theme === 'dark' ? t('shell.theme.light') : t('shell.theme.dark')}
    >
      {theme === 'dark' ? (
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" width="16" height="16">
          <circle cx="12" cy="12" r="5" />
          <line x1="12" y1="1" x2="12" y2="3" /><line x1="12" y1="21" x2="12" y2="23" />
          <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" /><line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
          <line x1="1" y1="12" x2="3" y2="12" /><line x1="21" y1="12" x2="23" y2="12" />
          <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" /><line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
        </svg>
      ) : (
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" width="16" height="16">
          <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
        </svg>
      )}
    </button>
  );

  const footerControls = (
    <div className="sidebar-footer-controls">
      {productRole !== null && (
        <span className="badge badge-neutral shell-product-role" title={t('shell.role.hint')}>
          {productRole === 'admin' ? t('shell.role.admin') : t('shell.role.user')}
        </span>
      )}
      {themeToggle}
    </div>
  );

  return (
    <AppShell
      brandTitle="Pymes SaaS"
      brandSubtitle={sentenceCase(t('shell.brand.subtitle'))}
      sections={sections}
      footerContent={footerControls}
    >
      {children}
    </AppShell>
  );
}
