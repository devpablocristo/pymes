/**
 * Ajustes — solo configuración del producto (preferencias, apariencia, integraciones, etc.).
 * El trabajo operativo del negocio vive en el menú lateral / módulos, no acá.
 */
import { useQuery } from '@tanstack/react-query';
import { useSearch } from '@devpablocristo/platform-search';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { SectionHubPage } from '@devpablocristo/platform-ui-section-hub';
import '@devpablocristo/platform-ui-section-hub/styles.css';
import { HeaderMenu } from '../components/HeaderMenu';
import { cleanHeaderMenuLabel } from '../components/headerMenuLabels';
import { usePageSearch } from '../components/PageSearch';
import { getSession } from '../lib/api';
import { queryKeys } from '../lib/queryKeys';
import { tenantLink, useActiveTenantSlug } from '../lib/tenantSlug';
import {
  NON_ADMIN_SECTIONS,
  SETTING_SECTIONS,
  sectionFromSearchParam,
  type SettingsSection,
  type SettingsSectionCard,
} from './SettingsHubPage.model';
import { SettingsHubSectionContent } from './SettingsHubSectionContent';
import './SettingsHubPage.css';

export function SettingsHubPage() {
  const navigate = useNavigate();
  const tenantSlug = useActiveTenantSlug();
  const settingsSearch = usePageSearch();
  const sectionTextFn = useCallback((s: SettingsSectionCard) => `${s.label} ${s.desc}`, []);
  const sessionQuery = useQuery({
    queryKey: queryKeys.session.current,
    queryFn: () => getSession(),
    staleTime: 5 * 60_000,
  });
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedSection = searchParams.get('section');
  const settingsReturn = useMemo(
    () => resolveSettingsReturn(searchParams, tenantSlug),
    [searchParams, tenantSlug],
  );
  const buildSearchParams = useCallback(
    (nextSection: Exclude<SettingsSection, null> | null): URLSearchParams => {
      const next = new URLSearchParams();
      if (nextSection) {
        next.set('section', nextSection);
      }
      if (settingsReturn) {
        next.set('returnLabel', settingsReturn.label);
        next.set('returnTo', settingsReturn.href);
      }
      return next;
    },
    [settingsReturn],
  );
  const waitingForAdminSection = !sessionQuery.data && (requestedSection === 'rbac' || requestedSection === 'audit');
  const isAccountAdmin = sessionQuery.data?.auth.product_role === 'admin';
  const availableSections = useMemo(() => {
    if (!sessionQuery.data) {
      return NON_ADMIN_SECTIONS;
    }
    if (isAccountAdmin) {
      return SETTING_SECTIONS;
    }
    return NON_ADMIN_SECTIONS;
  }, [isAccountAdmin, sessionQuery.data]);
  const filteredSections = useSearch(availableSections, sectionTextFn, settingsSearch);
  const [section, setSection] = useState<SettingsSection>(() =>
    sectionFromSearchParam(NON_ADMIN_SECTIONS, requestedSection),
  );

  useEffect(() => {
    if (requestedSection === 'crudUi') {
      navigate(tenantLink('/inventory/configure', tenantSlug), { replace: true });
    }
  }, [navigate, requestedSection, tenantSlug]);

  useEffect(() => {
    if (waitingForAdminSection) {
      setSection(null);
      return;
    }
    const nextSection = sectionFromSearchParam(availableSections, requestedSection);
    setSection(nextSection);
    if (requestedSection && !nextSection) {
      setSearchParams(buildSearchParams(null), { replace: true });
    }
  }, [availableSections, buildSearchParams, requestedSection, setSearchParams, waitingForAdminSection]);

  function openSection(id: Exclude<SettingsSection, null>): void {
    setSection(id);
    setSearchParams(buildSearchParams(id), { replace: true });
  }

  function goBackToGrid(): void {
    setSection(null);
    if (searchParams.get('section')) {
      setSearchParams(buildSearchParams(null), { replace: true });
    }
  }

  const activeSectionCard = availableSections.find((item) => item.id === section) ?? null;

  return (
    <div className="page-stack stg">
      <div className="page-layout__header-top-row">
        <HeaderMenu items={settingsReturn ? [settingsReturn] : []} />
      </div>
      <SectionHubPage
        pageTitle="Ajustes"
        pageLead=""
        sections={availableSections}
        visibleSections={filteredSections}
        emptyState={
          <div className="card">
            <p className="text-secondary u-m-0">No hay secciones de ajustes que coincidan con la búsqueda actual.</p>
          </div>
        }
        activeSectionId={section}
        onOpenSection={openSection}
        onBack={goBackToGrid}
        backLabel={activeSectionCard ? '← Volver a Ajustes' : 'Volver'}
      >
        <SettingsHubSectionContent
          section={section}
          isAccountAdmin={isAccountAdmin}
          tenantId={sessionQuery.data?.auth.org_id}
          membershipRole={sessionQuery.data?.membership?.role}
        />
      </SectionHubPage>
    </div>
  );
}

export default SettingsHubPage;

function resolveSettingsReturn(searchParams: URLSearchParams, tenantSlug: string | null) {
  const label = searchParams.get('returnLabel')?.trim();
  const href = searchParams.get('returnTo')?.trim();
  if (!label || !href || label.length > 80) return null;
  if (!href.startsWith('/') || href.startsWith('//')) return null;
  if (tenantSlug && href !== `/${tenantSlug}` && !href.startsWith(`/${tenantSlug}/`)) return null;
  return { label: cleanHeaderMenuLabel(label), href };
}
