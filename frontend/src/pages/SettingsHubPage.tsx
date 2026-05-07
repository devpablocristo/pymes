/**
 * Ajustes — solo configuración del producto (preferencias, apariencia, integraciones, etc.).
 * El trabajo operativo del negocio vive en el menú lateral / módulos, no acá.
 */
import { useQuery } from '@tanstack/react-query';
import { useSearch } from '@devpablocristo/modules-search';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { SectionHubPage } from '@devpablocristo/modules-ui-section-hub';
import '@devpablocristo/modules-ui-section-hub/styles.css';
import { HeaderMenu } from '../components/HeaderMenu';
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
    queryFn: getSession,
    staleTime: 5 * 60_000,
  });
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedSection = searchParams.get('section');
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
      setSearchParams({}, { replace: true });
    }
  }, [availableSections, requestedSection, setSearchParams, waitingForAdminSection]);

  function openSection(id: Exclude<SettingsSection, null>): void {
    setSection(id);
    setSearchParams({ section: id }, { replace: true });
  }

  function goBackToGrid(): void {
    setSection(null);
    if (searchParams.get('section')) {
      setSearchParams({}, { replace: true });
    }
  }

  const activeSectionCard = availableSections.find((item) => item.id === section) ?? null;

  return (
    <div className="page-stack stg">
      <div className="page-layout__header-top-row">
        <HeaderMenu />
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
          tenantId={sessionQuery.data?.auth.tenant_id}
          membershipRole={sessionQuery.data?.membership?.role}
        />
      </SectionHubPage>
    </div>
  );
}

export default SettingsHubPage;
