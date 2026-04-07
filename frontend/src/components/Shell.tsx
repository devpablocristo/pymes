import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { AppShell, type AppShellNavItem, type AppShellNavSection } from '../shared/frontendShell';
import {
  adminIcon,
  beautyIcon,
  bellIcon,
  bikeIcon,
  calendarIcon,
  carIcon,
  chartIcon,
  chatIcon,
  clockIcon,
  dashboardIcon,
  documentIcon,
  globeIcon,
  specialtiesIcon,
  teachersIcon,
  utensilsIcon,
  wrenchIcon,
} from './ShellIcons';
import { loadModuleCatalog } from '../lib/moduleCatalogLoader';
import { useI18n } from '../lib/i18n';
import { getVisibleModuleIds } from '../lib/profileFilters';
import { getTenantProfile } from '../lib/tenantProfile';
import { vocab } from '../lib/vocabulary';

type ModuleGroup = {
  id: string;
  label: string;
};

type ModuleListItem = {
  id: string;
  group: string;
  navLabel: string;
  icon: string;
  customRoute?: string;
};
function Glyph({ label }: { label: string }) {
  return <span className="sidebar-token">{label}</span>;
}

export function Shell({ children }: { children: ReactNode }) {
  const { t, localizeUiText, sentenceCase } = useI18n();
  const [catalog, setCatalog] = useState<{ groups: ModuleGroup[]; modules: ModuleListItem[] }>({
    groups: [],
    modules: [],
  });

  useEffect(() => {
    let cancelled = false;
    void loadModuleCatalog().then((mod) => {
      if (!cancelled) {
        setCatalog({
          groups: mod.moduleGroups.map((group) => ({ id: group.id, label: group.label })),
          modules: mod.moduleList.map((module) => ({
            id: module.id,
            group: module.group,
            navLabel: module.navLabel,
            icon: module.icon,
            customRoute: module.customRoute,
          })),
        });
      }
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const mainNav = useMemo<AppShellNavItem[]>(() => {
    const items: AppShellNavItem[] = [
      { to: '/dashboard', label: t('shell.nav.dashboard'), icon: chartIcon },
      { to: '/calendar', label: t('shell.nav.calendar'), icon: calendarIcon },
      { to: '/chat', label: t('shell.nav.chat'), icon: chatIcon },
      { to: '/notifications', label: t('shell.nav.notifications'), icon: bellIcon },
      { to: '/invoices', label: t('shell.nav.invoices'), icon: documentIcon },
      { to: '/whatsapp/inbox', label: t('shell.nav.whatsappInbox'), icon: chatIcon },
      { to: '/whatsapp/campaigns', label: t('shell.nav.whatsappCampaigns'), icon: chatIcon },
      { to: '/crypto', label: t('shell.nav.crypto'), icon: chartIcon },
      { to: '/ui', label: t('shell.nav.uiComponents'), icon: dashboardIcon },
      { to: '/settings', label: t('shell.nav.settings'), icon: adminIcon },
    ];
    return items;
  }, [t]);

  const professionalsNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: '/modules/teachers', label: t('shell.nav.teachers'), icon: teachersIcon },
      { to: '/modules/specialties', label: t('shell.nav.teachersSpecialties'), icon: specialtiesIcon },
      { to: '/modules/intakes', label: t('shell.nav.teachersIntakes'), icon: documentIcon },
      { to: '/modules/sessions', label: t('shell.nav.teachersSessions'), icon: clockIcon },
    ],
    [t],
  );

  const workshopsNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: '/modules/workshopVehicles', label: t('shell.nav.autoRepairVehicles'), icon: carIcon },
      { to: '/modules/workOrders', label: t('shell.nav.autoRepairOrders'), icon: documentIcon },
    ],
    [t],
  );

  const bikeShopNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: '/workshops/bike-shop/orders', label: t('shell.nav.bikeOrders'), icon: documentIcon },
    ],
    [t],
  );

  const beautyNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: '/modules/employees', label: t('shell.nav.beautyStaff'), icon: teachersIcon },
    ],
    [t],
  );

  const restaurantsNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: '/modules/restaurantDiningAreas', label: t('shell.nav.restaurantAreas'), icon: dashboardIcon },
      { to: '/modules/restaurantDiningTables', label: t('shell.nav.restaurantTables'), icon: utensilsIcon },
      { to: '/restaurants/dining/sessions', label: t('shell.nav.restaurantSessions'), icon: clockIcon },
    ],
    [t],
  );

  const sections = useMemo(() => {
    const visibleIds = getVisibleModuleIds();
    const profile = getTenantProfile();
    const vertical = profile?.vertical ?? 'none';
    const baseNav: AppShellNavItem[] = [...mainNav];

    if (profile?.usesScheduling) {
      baseNav.splice(2, 0, {
        to: '/scheduling/public-preview',
        label: t('shell.nav.schedulingPublic'),
        icon: globeIcon,
      });
    }

    const moduleNav = catalog.groups
      .map<AppShellNavSection>((group) => ({
        label: localizeUiText(group.label),
        items: catalog.modules
          .filter((module) => module.group === group.id && visibleIds.has(module.id))
          .sort((left, right) =>
            localizeUiText(vocab(left.navLabel)).localeCompare(localizeUiText(vocab(right.navLabel))),
          )
          .map((module) => ({
            to: module.customRoute ?? `/modules/${module.id}`,
            label: localizeUiText(vocab(module.navLabel)),
            icon: <Glyph label={module.icon} />,
          })),
      }))
      .filter((section) => section.items.length > 0);

    const result: AppShellNavSection[] = [{ label: sentenceCase(t('shell.sections.base')), items: baseNav }];
    if (vertical === 'professionals') {
      result.push({ label: sentenceCase(t('shell.sections.professionals')), items: professionalsNav });
    }
    if (vertical === 'workshops') {
      result.push({ label: sentenceCase(t('shell.sections.workshops')), items: workshopsNav });
    }
    if (vertical === 'bike_shop') {
      result.push({ label: sentenceCase(t('shell.sections.bikeShop')), items: bikeShopNav });
    }
    if (vertical === 'beauty') {
      result.push({ label: sentenceCase(t('shell.sections.beauty')), items: beautyNav });
    }
    if (vertical === 'restaurants') {
      result.push({ label: sentenceCase(t('shell.sections.restaurants')), items: restaurantsNav });
    }
    result.push(...moduleNav);
    return result;
  }, [
    beautyNav,
    bikeShopNav,
    catalog.groups,
    catalog.modules,
    localizeUiText,
    mainNav,
    professionalsNav,
    restaurantsNav,
    sentenceCase,
    t,
    workshopsNav,
  ]);

  return (
    <AppShell
      brandTitle="Pymes SaaS"
      brandSubtitle={sentenceCase(t('shell.brand.subtitle'))}
      sections={sections}
      searchPlaceholder={t('shell.search.placeholder')}
      skipLinkLabel={t('shell.skipLink')}
    >
      {children}
    </AppShell>
  );
}
