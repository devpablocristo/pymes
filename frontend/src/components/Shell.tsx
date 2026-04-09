import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { AppShell, type AppShellNavItem, type AppShellNavSection } from '../shared/frontendShell';
import { dotIcon } from './ShellIcons';
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
// Decisión de producto: TODOS los items del sidebar usan el mismo glyph
// (un círculo simple). La diferenciación es por label, no por icono. Esto
// elimina ruido visual y forza al usuario a leer la etiqueta. Si en el
// futuro se vuelve a iconos por concepto, restaurar el mapeo aquí.

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

  // Sidebar dividido en secciones lógicas (estilo Linear / Notion / Slack):
  // - "Inicio" arriba (landing del usuario al entrar).
  // - "Día a día" agrupa lo que se toca todo el día.
  // - "Comercial" lo transaccional / dinero.
  // - "WhatsApp" en su propia sección porque es un canal completo aparte.
  // - Verticales y módulos dinámicos en el medio.
  // - "Sistema" al final (settings, convención de la industria).

  const homeNav = useMemo<AppShellNavItem[]>(
    () => [{ to: '/dashboard', label: t('shell.nav.dashboard'), icon: dotIcon }],
    [t],
  );

  const dailyNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: '/agenda', label: t('shell.nav.calendar'), icon: dotIcon },
      { to: '/chat', label: t('shell.nav.chat'), icon: dotIcon },
      { to: '/notifications', label: t('shell.nav.notifications'), icon: dotIcon },
    ],
    [t],
  );

  const commercialNav = useMemo<AppShellNavItem[]>(
    () => [{ to: '/invoices', label: t('shell.nav.invoices'), icon: dotIcon }],
    [t],
  );

  // WhatsApp: NO tocar items ni paths ni labels. Otro agente maneja whatsapp.
  // Solo se mueven a su propia sección sidebar para coherencia visual.
  const whatsappNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: '/whatsapp/inbox', label: t('shell.nav.whatsappInbox'), icon: dotIcon },
      { to: '/whatsapp/campaigns', label: t('shell.nav.whatsappCampaigns'), icon: dotIcon },
    ],
    [t],
  );

  const systemNav = useMemo<AppShellNavItem[]>(
    () => [{ to: '/settings', label: t('shell.nav.settings'), icon: dotIcon }],
    [t],
  );

  const professionalsNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: '/modules/teachers', label: t('shell.nav.teachers'), icon: dotIcon },
      { to: '/modules/specialties', label: t('shell.nav.teachersSpecialties'), icon: dotIcon },
      { to: '/modules/intakes', label: t('shell.nav.teachersIntakes'), icon: dotIcon },
      { to: '/modules/sessions', label: t('shell.nav.teachersSessions'), icon: dotIcon },
    ],
    [t],
  );

  const workshopsNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: '/modules/workshopVehicles', label: t('shell.nav.autoRepairVehicles'), icon: dotIcon },
      { to: '/modules/carWorkOrders', label: t('shell.nav.autoRepairOrders'), icon: dotIcon },
    ],
    [t],
  );

  const bikeShopNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: '/workshops/bike-shop/orders', label: t('shell.nav.bikeOrders'), icon: dotIcon },
    ],
    [t],
  );

  const beautyNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: '/modules/employees', label: t('shell.nav.beautyStaff'), icon: dotIcon },
    ],
    [t],
  );

  const restaurantsNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: '/modules/restaurantDiningAreas', label: t('shell.nav.restaurantAreas'), icon: dotIcon },
      { to: '/modules/restaurantDiningTables', label: t('shell.nav.restaurantTables'), icon: dotIcon },
      { to: '/restaurants/dining/sessions', label: t('shell.nav.restaurantSessions'), icon: dotIcon },
    ],
    [t],
  );

  const sections = useMemo(() => {
    const visibleIds = getVisibleModuleIds();
    const profile = getTenantProfile();
    const vertical = profile?.vertical ?? 'none';

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
            icon: dotIcon,
          })),
      }))
      .filter((section) => section.items.length > 0);

    const result: AppShellNavSection[] = [
      { label: sentenceCase(t('shell.sections.home')), items: homeNav },
      { label: sentenceCase(t('shell.sections.daily')), items: dailyNav },
      { label: sentenceCase(t('shell.sections.commercial')), items: commercialNav },
      { label: sentenceCase(t('shell.sections.whatsapp')), items: whatsappNav },
    ];

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
    // Sistema (Settings) al final, convención de Linear / Slack / Notion.
    result.push({ label: sentenceCase(t('shell.sections.system')), items: systemNav });
    return result;
  }, [
    beautyNav,
    bikeShopNav,
    catalog.groups,
    catalog.modules,
    commercialNav,
    dailyNav,
    homeNav,
    localizeUiText,
    professionalsNav,
    restaurantsNav,
    sentenceCase,
    systemNav,
    t,
    whatsappNav,
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
