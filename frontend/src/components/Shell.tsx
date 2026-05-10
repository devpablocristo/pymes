import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { useOrganization } from '@clerk/react';
import { AppShell, type AppShellNavItem, type AppShellNavSection } from '../shared/frontendShell';
import { loadModuleCatalog } from '../lib/moduleCatalogLoader';
import { useI18n } from '../lib/i18n';
import { getVisibleModuleIds } from '../lib/profileFilters';
import { getTenantProfile } from '../lib/tenantProfile';
import { vocab } from '../lib/vocabulary';
import { toCrudResourceSlug } from '../crud/crudResourceSlug';
import { tenantLink, useTenantSlug } from '../lib/tenantSlug';
import { clerkEnabled } from '../lib/auth';
import logoUrl from '../assets/logo.svg';
import logoDarkUrl from '../assets/logo-dark.svg';
import isoUrl from '../assets/iso.svg';

/* Componente nulo: usa useOrganization (solo seguro dentro de ClerkProvider)
   y notifica el nombre vía callback. Se monta condicionalmente. */
function ClerkOrgNameSync({ onName }: { onName: (name: string) => void }) {
  const { organization } = useOrganization();
  useEffect(() => {
    onName(organization?.name?.trim() ?? '');
  }, [organization?.name, onName]);
  return null;
}

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

// Tabler icon helper
function ti(name: string): ReactNode {
  return <i className={`ti ti-${name}`} aria-hidden="true" />;
}

// Mapeo de módulo ID → icono Tabler
const MODULE_ICON_MAP: Record<string, string> = {
  customers:  'users',
  suppliers:  'building-store',
  products:   'package',
  services:   'scissors',
  quotes:     'file-description',
  sales:      'shopping-cart',
  purchases:  'arrows-exchange',
  inventory:  'box',
  cashflow:   'chart-bar',
  reports:    'chart-line',
};

export function Shell({ children }: { children: ReactNode }) {
  const { t, localizeUiText, sentenceCase } = useI18n();
  const slug = useTenantSlug();
  const [clerkOrgName, setClerkOrgName] = useState('');
  const profileOrgName = getTenantProfile()?.businessName ?? '';
  const orgName = clerkOrgName || profileOrgName;
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

  const homeNav = useMemo<AppShellNavItem[]>(
    () => [{ to: link('/dashboard'), label: t('shell.nav.dashboard'), icon: ti('layout-dashboard') }],
    [t, slug],
  );

  const dailyNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: link('/agenda'),        label: t('shell.nav.calendar'),      icon: ti('calendar-event') },
      { to: link('/chat'),          label: t('shell.nav.chat'),           icon: ti('robot') },
      { to: link('/notifications'), label: t('shell.nav.notifications'),  icon: ti('bell') },
    ],
    [t, slug],
  );

  const commercialNav = useMemo<AppShellNavItem[]>(
    () => [{ to: link('/invoices'), label: t('shell.nav.invoices'), icon: ti('receipt-2') }],
    [t, slug],
  );

  const whatsappNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: link('/customer-messaging/inbox'),     label: t('shell.nav.whatsappInbox'),     icon: ti('brand-whatsapp') },
      { to: link('/customer-messaging/campaigns'), label: t('shell.nav.whatsappCampaigns'), icon: ti('speakerphone') },
    ],
    [t, slug],
  );

  const professionalsNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: link('/teachers'),    label: t('shell.nav.teachers'),           icon: ti('school') },
      { to: link('/specialties'), label: t('shell.nav.teachersSpecialties'), icon: ti('certificate') },
      { to: link('/intakes'),     label: t('shell.nav.teachersIntakes'),     icon: ti('door-enter') },
      { to: link('/sessions'),    label: t('shell.nav.teachersSessions'),    icon: ti('calendar-time') },
    ],
    [t, slug],
  );

  const workshopsNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: link(`/${toCrudResourceSlug('workshopVehicles')}`),     label: t('shell.nav.autoRepairVehicles'), icon: ti('car') },
      { to: link(`/${toCrudResourceSlug('carWorkOrders')}/list`),   label: t('shell.nav.autoRepairOrders'),   icon: ti('tool') },
    ],
    [t, slug],
  );

  const bikeShopNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: link(`/${toCrudResourceSlug('bikeWorkOrders')}/list`), label: t('shell.nav.bikeOrders'), icon: ti('bike') },
    ],
    [t, slug],
  );

  const beautyNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: link('/employees'), label: t('shell.nav.beautyStaff'), icon: ti('users-group') },
    ],
    [t, slug],
  );

  const restaurantsNav = useMemo<AppShellNavItem[]>(
    () => [
      { to: link(`/${toCrudResourceSlug('restaurantDiningAreas')}`),   label: t('shell.nav.restaurantAreas'),    icon: ti('layout-2') },
      { to: link(`/${toCrudResourceSlug('restaurantDiningTables')}`),  label: t('shell.nav.restaurantTables'),   icon: ti('table') },
      { to: link('/restaurants/dining/sessions'),                      label: t('shell.nav.restaurantSessions'), icon: ti('clock') },
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
        icon: ti(MODULE_ICON_MAP[module.id] ?? 'circle-dot'),
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
            icon: ti(MODULE_ICON_MAP[module.id] ?? 'circle-dot'),
          })),
      }))
      .filter((section) => section.items.length > 0);

    const result: AppShellNavSection[] = [
      { label: sentenceCase(t('shell.sections.home')),       items: homeNav },
      { label: sentenceCase(t('shell.sections.daily')),      items: dailyNav },
      { label: sentenceCase(t('shell.sections.commercial')), items: [...commercialNav, ...commercialModuleItems] },
      { label: sentenceCase(t('shell.sections.whatsapp')),   items: whatsappNav },
    ];

    if (vertical === 'professionals') {
      result.push({ label: sentenceCase(t('shell.sections.professionals')), items: professionalsNav });
    }
    if (vertical === 'workshops') {
      if (subVertical === 'bike_shop') {
        result.push({ label: sentenceCase(t('shell.sections.bikeShop')),  items: bikeShopNav });
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

  const brandLogo = (
    <>
      <img src={logoUrl} alt="Wukomo" className="brand-logo-full brand-logo-full--light" style={{ height: '22px', display: 'block' }} />
      <img src={logoDarkUrl} alt="Wukomo" className="brand-logo-full brand-logo-full--dark" style={{ height: '22px', display: 'block' }} />
      <img src={isoUrl} alt="W" className="brand-logo-iso" />
    </>
  );

  return (
    <>
      {clerkEnabled && <ClerkOrgNameSync onName={setClerkOrgName} />}
      <AppShell
        brandTitle={brandLogo}
        brandSubtitle={orgName || sentenceCase(t('shell.brand.subtitle'))}
      sections={sections}
      searchPlaceholder={t('shell.search.placeholder')}
      skipLinkLabel={t('shell.skipLink')}
    >
      {children}
    </AppShell>
    </>
  );
}
