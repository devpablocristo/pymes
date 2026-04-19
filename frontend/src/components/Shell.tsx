import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { AppShell, type AppShellNavItem, type AppShellNavSection } from '../shared/frontendShell';
import { dotIcon } from './ShellIcons';
import { loadModuleCatalog } from '../lib/moduleCatalogLoader';
import { useI18n } from '../lib/i18n';
import { getVisibleModuleIds } from '../lib/profileFilters';
import { getTenantProfile } from '../lib/tenantProfile';
import { vocab } from '../lib/vocabulary';
import { toCrudResourceSlug } from '../crud/crudResourceSlug';
import { tenantLink, useTenantSlug } from '../lib/tenantSlug';

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

const PRIMARY_SIDEBAR_MODULE_IDS = new Set([
  'customers',
  'suppliers',
  'products',
  'services',
  'quotes',
  'sales',
  'purchases',
  'inventory',
  'cashflow',
  'reports',
]);
// Decisión de producto: TODOS los items del sidebar usan el mismo glyph
// (un círculo simple). La diferenciación es por label, no por icono. Esto
// elimina ruido visual y forza al usuario a leer la etiqueta. Si en el
// futuro se vuelve a iconos por concepto, restaurar el mapeo aquí.

export function Shell({ children }: { children: ReactNode }) {
  const { t, localizeUiText, sentenceCase } = useI18n();
  const slug = useTenantSlug();
  const link = (path: string) => tenantLink(path, slug);
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

  // Sidebar dividido en secciones lógicas:
  // - "Inicio" arriba.
  // - "Día a día" para operación frecuente.
  // - "Comercial" para lo transaccional.
  // - "WhatsApp" como entrada a customer messaging sobre ese canal.
  // - Verticales y módulos dinámicos en el medio.
  // - "Sistema" al final.

  const homeNav = useMemo<AppShellNavItem[]>(
    () => [{ to: link('/dashboard'), label: t('shell.nav.dashboard'), icon: dotIcon }],
    [t, slug],
  );

  const dailyNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: link('/agenda'), label: t('shell.nav.calendar'), icon: dotIcon },
      { to: link('/chat'), label: t('shell.nav.chat'), icon: dotIcon },
      { to: link('/notifications'), label: t('shell.nav.notifications'), icon: dotIcon },
    ],
    [t, slug],
  );

  const commercialNav = useMemo<AppShellNavItem[]>(
    () => [{ to: link('/invoices'), label: t('shell.nav.invoices'), icon: dotIcon }],
    [t, slug],
  );

  const whatsappNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: link('/customer-messaging/inbox'), label: t('shell.nav.whatsappInbox'), icon: dotIcon },
      { to: link('/customer-messaging/campaigns'), label: t('shell.nav.whatsappCampaigns'), icon: dotIcon },
    ],
    [t, slug],
  );

  const professionalsNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: link('/teachers'), label: t('shell.nav.teachers'), icon: dotIcon },
      { to: link('/specialties'), label: t('shell.nav.teachersSpecialties'), icon: dotIcon },
      { to: link('/intakes'), label: t('shell.nav.teachersIntakes'), icon: dotIcon },
      { to: link('/sessions'), label: t('shell.nav.teachersSessions'), icon: dotIcon },
    ],
    [t, slug],
  );

  const workshopsNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: link(`/${toCrudResourceSlug('workshopVehicles')}`), label: t('shell.nav.autoRepairVehicles'), icon: dotIcon },
      { to: link(`/${toCrudResourceSlug('carWorkOrders')}/list`), label: t('shell.nav.autoRepairOrders'), icon: dotIcon },
    ],
    [t, slug],
  );

  const bikeShopNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: link(`/${toCrudResourceSlug('bikeWorkOrders')}/list`), label: t('shell.nav.bikeOrders'), icon: dotIcon },
    ],
    [t, slug],
  );

  const beautyNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: link('/employees'), label: t('shell.nav.beautyStaff'), icon: dotIcon },
    ],
    [t, slug],
  );

  const restaurantsNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: link(`/${toCrudResourceSlug('restaurantDiningAreas')}`), label: t('shell.nav.restaurantAreas'), icon: dotIcon },
      { to: link(`/${toCrudResourceSlug('restaurantDiningTables')}`), label: t('shell.nav.restaurantTables'), icon: dotIcon },
      { to: link('/restaurants/dining/sessions'), label: t('shell.nav.restaurantSessions'), icon: dotIcon },
    ],
    [t, slug],
  );

  const sections = useMemo(() => {
    const visibleIds = getVisibleModuleIds();
    const profile = getTenantProfile();
    const vertical = profile?.vertical ?? 'none';
    const subVertical = profile?.subVertical ?? null;

    const commercialModuleItems = catalog.modules
      .filter(
        (module) =>
          module.group === 'commercial' &&
          visibleIds.has(module.id) &&
          PRIMARY_SIDEBAR_MODULE_IDS.has(module.id),
      )
      .sort((left, right) =>
        localizeUiText(vocab(left.navLabel)).localeCompare(localizeUiText(vocab(right.navLabel))),
      )
      .map((module) => ({
        to: module.customRoute ? link(module.customRoute) : link(`/${toCrudResourceSlug(module.id)}`),
        label: localizeUiText(vocab(module.navLabel)),
        icon: dotIcon,
      }));

    const moduleNav = catalog.groups
      .map<AppShellNavSection>((group) => ({
        label: localizeUiText(group.label),
        items: catalog.modules
          .filter(
            (module) =>
              group.id !== 'commercial' &&
              module.group === group.id &&
              visibleIds.has(module.id) &&
              PRIMARY_SIDEBAR_MODULE_IDS.has(module.id),
          )
          .sort((left, right) =>
            localizeUiText(vocab(left.navLabel)).localeCompare(localizeUiText(vocab(right.navLabel))),
          )
          .map((module) => ({
            to: module.customRoute ? link(module.customRoute) : link(`/${toCrudResourceSlug(module.id)}`),
            label: localizeUiText(vocab(module.navLabel)),
            icon: dotIcon,
          })),
      }))
      .filter((section) => section.items.length > 0);

    const result: AppShellNavSection[] = [
      { label: sentenceCase(t('shell.sections.home')), items: homeNav },
      { label: sentenceCase(t('shell.sections.daily')), items: dailyNav },
      { label: sentenceCase(t('shell.sections.commercial')), items: [...commercialNav, ...commercialModuleItems] },
      { label: sentenceCase(t('shell.sections.whatsapp')), items: whatsappNav },
    ];

    if (vertical === 'professionals') {
      result.push({ label: sentenceCase(t('shell.sections.professionals')), items: professionalsNav });
    }
    if (vertical === 'workshops') {
      if (subVertical === 'bike_shop') {
        result.push({ label: sentenceCase(t('shell.sections.bikeShop')), items: bikeShopNav });
      } else {
        result.push({ label: sentenceCase(t('shell.sections.workshops')), items: workshopsNav });
      }
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
    commercialNav,
    dailyNav,
    homeNav,
    localizeUiText,
    professionalsNav,
    restaurantsNav,
    sentenceCase,
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
