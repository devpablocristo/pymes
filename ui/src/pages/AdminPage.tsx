import { FormEvent, useCallback, useEffect, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { PageLayout } from '../components/PageLayout';
import { usePageSearch } from '../components/PageSearch';
import { useSearch } from '@devpablocristo/platform-search';
import {
  downloadAuditExportCsv,
  getAuditEntries,
  getSession,
  getTenantSettings,
  updateTenantSettings,
} from '../lib/api';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { queryKeys } from '../lib/queryKeys';
import { getTheme, toggleTheme } from '../lib/theme';
import { syncTenantProfileFromSettings } from '../lib/tenantProfile';
import type { AuditEntry } from '../lib/types';
import { AdminRbacSection } from './AdminRbacSection';
import { AdminAppearanceSection } from './AdminAppearanceSection';
import { AdminAuditSection } from './AdminAuditSection';
import { buildPayload, settingsToForm, type AdminSection, type TenantFormState } from './AdminPage.model';
import { AdminWorkspaceSettingsForm } from './AdminWorkspaceSettingsForm';

type AdminPageProps = {
  section?: AdminSection;
  embedded?: boolean;
};

const ADMIN_SECTION_META: Record<AdminSection, { title: string; lead: string }> = {
  all: {
    title: 'Administración',
    lead: 'Configuración del espacio y registro de actividad',
  },
  appearance: {
    title: 'Apariencia',
    lead: 'Tema visual y preferencias globales del espacio de trabajo.',
  },
  workspace: {
    title: 'Negocio',
    lead: 'Datos base, monedas, correlativos y configuración operativa del espacio.',
  },
  rbac: {
    title: 'Roles y permisos',
    lead: 'Asignación de accesos y gestión de permisos administrativos.',
  },
  audit: {
    title: 'Auditoría',
    lead: 'Registro de actividad y exportación de eventos del espacio.',
  },
};

export function AdminPage({ section = 'all', embedded = false }: AdminPageProps = {}) {
  const queryClient = useQueryClient();
  const [uiTheme, setUiTheme] = useState(getTheme);
  const [form, setForm] = useState<TenantFormState | null>(null);
  const adminSearch = usePageSearch();
  const auditTextFn = useCallback(
    (a: AuditEntry) => `${a.action} ${a.resource_type} ${a.resource_id ?? ''} ${a.actor ?? ''}`,
    [],
  );
  const tenantQuery = useQuery({
    queryKey: queryKeys.tenant.settings,
    queryFn: () => getTenantSettings(),
    staleTime: 60_000,
  });
  const auditQuery = useQuery({
    queryKey: queryKeys.audit.entries,
    queryFn: async () => {
      const audit = await getAuditEntries();
      return audit.items ?? [];
    },
    staleTime: 30_000,
  });
  const sessionQuery = useQuery({
    queryKey: queryKeys.session.current,
    queryFn: () => getSession(),
    staleTime: 5 * 60_000,
  });
  const settings = tenantQuery.data ?? null;
  const activity = auditQuery.data ?? [];
  const filteredActivity = useSearch(activity, auditTextFn, adminSearch);
  const [error, setError] = useState('');
  const loading = tenantQuery.isPending || auditQuery.isPending;
  const [saving, setSaving] = useState(false);
  const sessionTenantId = sessionQuery.data?.auth.org_id ?? '';
  const isConsoleAdmin = sessionQuery.data?.auth.product_role === 'admin';
  const [auditExportBusy, setAuditExportBusy] = useState(false);

  const loadError = tenantQuery.isError
    ? formatFetchErrorForUser(tenantQuery.error, 'No pudimos conectar con el servidor. Verificá tu red.')
    : auditQuery.isError
      ? formatFetchErrorForUser(auditQuery.error, 'No pudimos conectar con el servidor. Verificá tu red.')
      : '';

  useEffect(() => {
    if (tenantQuery.data && form === null) {
      setForm(settingsToForm(tenantQuery.data));
    }
  }, [tenantQuery.data, form]);

  function handleAppearanceToggle(): void {
    const next = toggleTheme();
    setUiTheme(next);
  }

  async function handleAuditExportCsv(): Promise<void> {
    setAuditExportBusy(true);
    try {
      await downloadAuditExportCsv();
      setError('');
    } catch (err) {
      setError(formatFetchErrorForUser(err, 'No se pudo descargar el CSV de auditoría.'));
    } finally {
      setAuditExportBusy(false);
    }
  }

  function updateField<K extends keyof TenantFormState>(key: K, value: TenantFormState[K]): void {
    setForm((prev) => (prev ? { ...prev, [key]: value } : prev));
  }

  function updateCurrencyRow(index: number, value: string): void {
    setForm((prev) => {
      if (!prev) return prev;
      const next = [...prev.currencies];
      next[index] = value;
      return { ...prev, currencies: next };
    });
  }

  function addCurrencyRow(): void {
    setForm((prev) => (prev ? { ...prev, currencies: [...prev.currencies, ''] } : prev));
  }

  function removeCurrencyRow(index: number): void {
    setForm((prev) => {
      if (!prev || prev.currencies.length <= 1) return prev;
      const next = prev.currencies.filter((_, i) => i !== index);
      return { ...prev, currencies: next };
    });
  }

  function moveCurrencyRow(index: number, delta: number): void {
    setForm((prev) => {
      if (!prev) return prev;
      const j = index + delta;
      if (j < 0 || j >= prev.currencies.length) return prev;
      const next = [...prev.currencies];
      [next[index], next[j]] = [next[j], next[index]];
      return { ...prev, currencies: next };
    });
  }

  async function onSubmit(event: FormEvent): Promise<void> {
    event.preventDefault();
    if (!settings || !form) return;
    const built = buildPayload(form);
    if ('error' in built) {
      setError(built.error);
      return;
    }
    setSaving(true);
    setError('');
    try {
      const updated = await updateTenantSettings(built);
      queryClient.setQueryData(queryKeys.tenant.settings, updated);
      setForm(settingsToForm(updated));
      syncTenantProfileFromSettings(updated);
    } catch (err) {
      setError(formatFetchErrorForUser(err, 'No pudimos conectar con el servidor. Verificá tu red.'));
    } finally {
      setSaving(false);
    }
  }

  function onResetForm(): void {
    if (tenantQuery.data) setForm(settingsToForm(tenantQuery.data));
    setError('');
  }

  const showAll = section === 'all';

  const content = (
    <>
      {(showAll || section === 'appearance') && (
        <AdminAppearanceSection uiTheme={uiTheme} onToggle={handleAppearanceToggle} />
      )}

      {(error || loadError) && <div className="alert alert-error">{error || loadError}</div>}

      {(showAll || section === 'workspace') && (
        <div className="card">
          <div className="card-header">
            <h2>Configuración del espacio</h2>
          </div>

          {loading && <div className="spinner" aria-label="Cargando" />}

          {!loading && settings && form && (
            <AdminWorkspaceSettingsForm
              settings={settings}
              form={form}
              saving={saving}
              onSubmit={(e) => void onSubmit(e)}
              onReset={onResetForm}
              updateField={updateField}
              updateCurrencyRow={updateCurrencyRow}
              addCurrencyRow={addCurrencyRow}
              removeCurrencyRow={removeCurrencyRow}
              moveCurrencyRow={moveCurrencyRow}
            />
          )}
        </div>
      )}

      {(showAll || section === 'rbac') && isConsoleAdmin && sessionTenantId ? (
        <AdminRbacSection tenantId={sessionTenantId} />
      ) : null}

      {(showAll || section === 'audit') && (
        <AdminAuditSection
          activity={activity}
          filteredActivity={filteredActivity}
          auditExportBusy={auditExportBusy}
          onExportCsv={() => void handleAuditExportCsv()}
        />
      )}
    </>
  );

  if (embedded) {
    return content;
  }

  const meta = ADMIN_SECTION_META[section];
  return (
    <PageLayout title={meta.title} lead={meta.lead}>
      {content}
    </PageLayout>
  );
}
