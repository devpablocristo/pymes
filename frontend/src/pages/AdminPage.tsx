import { FormEvent, useCallback, useEffect, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { PageLayout } from '../components/PageLayout';
import { usePageSearch } from '../components/PageSearch';
import { useSearch } from '@devpablocristo/modules-search';
import {
  downloadAuditExportCsv,
  getAuditEntries,
  getSession,
  getTenantSettings,
  updateTenantSettings,
} from '../lib/api';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { useI18n } from '../lib/i18n';
import { queryKeys } from '../lib/queryKeys';
import { getTheme, toggleTheme } from '../lib/theme';
import { syncTenantProfileFromSettings } from '../lib/tenantProfile';
import type { AuditEntry, TenantSettings, TenantSettingsUpdatePayload } from '../lib/types';
import { AdminRbacSection } from './AdminRbacSection';

function formatDateTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString('es-AR', {
      dateStyle: 'short',
      timeStyle: 'short',
    });
  } catch {
    return iso;
  }
}

function buildPayload(f: TenantFormState): TenantSettingsUpdatePayload | { error: string } {
  const tax = Number(f.tax_rate);
  if (!Number.isFinite(tax) || tax < 0) {
    return { error: 'El IVA debe ser un número mayor o igual a 0.' };
  }
  const reminder = Number(f.scheduling_reminder_hours);
  if (!Number.isFinite(reminder) || reminder < 0) {
    return { error: 'Las horas de recordatorio deben ser un número ≥ 0.' };
  }

  const seen = new Set<string>();
  const supported_currencies: string[] = [];
  for (const raw of f.currencies) {
    const c = raw.trim().toUpperCase();
    if (!c) continue;
    if (seen.has(c)) continue;
    seen.add(c);
    supported_currencies.push(c);
  }
  if (supported_currencies.length === 0) {
    return { error: 'Agregá al menos una moneda (código ISO, ej. ARS, USD).' };
  }

  return {
    supported_currencies,
    tax_rate: tax,
    quote_prefix: f.quote_prefix.trim(),
    sale_prefix: f.sale_prefix.trim(),
    allow_negative_stock: f.allow_negative_stock,
    purchase_prefix: f.purchase_prefix.trim(),
    return_prefix: f.return_prefix.trim(),
    credit_note_prefix: f.credit_note_prefix.trim(),
    business_name: f.business_name.trim(),
    business_tax_id: f.business_tax_id.trim(),
    business_address: f.business_address.trim(),
    business_phone: f.business_phone.trim(),
    business_email: f.business_email.trim(),
    vertical: f.vertical.trim(),
    wa_quote_template: f.wa_quote_template,
    wa_receipt_template: f.wa_receipt_template,
    wa_default_country_code: f.wa_default_country_code.trim(),
    scheduling_enabled: f.scheduling_enabled,
    scheduling_label: f.scheduling_label.trim(),
    scheduling_reminder_hours: reminder,
    default_rate_type: f.default_rate_type.trim(),
    auto_fetch_rates: f.auto_fetch_rates,
    show_dual_prices: f.show_dual_prices,
    bank_holder: f.bank_holder.trim(),
    bank_cbu: f.bank_cbu.trim(),
    bank_alias: f.bank_alias.trim(),
    bank_name: f.bank_name.trim(),
    show_qr_in_pdf: f.show_qr_in_pdf,
    wa_payment_template: f.wa_payment_template,
    wa_payment_link_template: f.wa_payment_link_template,
  };
}

function currenciesFromTenant(s: TenantSettings): string[] {
  if (Array.isArray(s.supported_currencies) && s.supported_currencies.length > 0) {
    return s.supported_currencies.map((c) => String(c).trim());
  }
  const cur = (s.currency ?? 'ARS').trim() || 'ARS';
  const sec = (s.secondary_currency ?? '').trim();
  return sec ? [cur, sec] : [cur];
}

type TenantFormState = {
  currencies: string[];
  tax_rate: string;
  quote_prefix: string;
  sale_prefix: string;
  allow_negative_stock: boolean;
  purchase_prefix: string;
  return_prefix: string;
  credit_note_prefix: string;
  business_name: string;
  business_tax_id: string;
  business_address: string;
  business_phone: string;
  business_email: string;
  vertical: string;
  wa_quote_template: string;
  wa_receipt_template: string;
  wa_default_country_code: string;
  scheduling_enabled: boolean;
  scheduling_label: string;
  scheduling_reminder_hours: string;
  default_rate_type: string;
  auto_fetch_rates: boolean;
  show_dual_prices: boolean;
  bank_holder: string;
  bank_cbu: string;
  bank_alias: string;
  bank_name: string;
  show_qr_in_pdf: boolean;
  wa_payment_template: string;
  wa_payment_link_template: string;
};

function settingsToForm(s: TenantSettings): TenantFormState {
  return {
    currencies: currenciesFromTenant(s),
    tax_rate: String(s.tax_rate ?? ''),
    quote_prefix: s.quote_prefix ?? '',
    sale_prefix: s.sale_prefix ?? '',
    allow_negative_stock: Boolean(s.allow_negative_stock),
    purchase_prefix: s.purchase_prefix ?? '',
    return_prefix: s.return_prefix ?? '',
    credit_note_prefix: s.credit_note_prefix ?? '',
    business_name: s.business_name ?? '',
    business_tax_id: s.business_tax_id ?? '',
    business_address: s.business_address ?? '',
    business_phone: s.business_phone ?? '',
    business_email: s.business_email ?? '',
    vertical: s.vertical ?? '',
    wa_quote_template: s.wa_quote_template ?? '',
    wa_receipt_template: s.wa_receipt_template ?? '',
    wa_default_country_code: s.wa_default_country_code ?? '',
    scheduling_enabled: Boolean(s.scheduling_enabled),
    scheduling_label: s.scheduling_label ?? '',
    scheduling_reminder_hours: String(s.scheduling_reminder_hours ?? ''),
    default_rate_type: s.default_rate_type ?? '',
    auto_fetch_rates: Boolean(s.auto_fetch_rates),
    show_dual_prices: Boolean(s.show_dual_prices),
    bank_holder: s.bank_holder ?? '',
    bank_cbu: s.bank_cbu ?? '',
    bank_alias: s.bank_alias ?? '',
    bank_name: s.bank_name ?? '',
    show_qr_in_pdf: Boolean(s.show_qr_in_pdf),
    wa_payment_template: s.wa_payment_template ?? '',
    wa_payment_link_template: s.wa_payment_link_template ?? '',
  };
}

export type AdminSection = 'all' | 'appearance' | 'workspace' | 'rbac' | 'audit';

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
  const { t } = useI18n();
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
    queryFn: getTenantSettings,
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
    queryFn: getSession,
    staleTime: 5 * 60_000,
  });

  const settings = tenantQuery.data ?? null;
  const activity = auditQuery.data ?? [];
  const filteredActivity = useSearch(activity, auditTextFn, adminSearch);
  const [error, setError] = useState('');
  const loading = tenantQuery.isPending || auditQuery.isPending;
  const [saving, setSaving] = useState(false);
  const sessionOrgId = sessionQuery.data?.auth.org_id ?? '';
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
        <div className="card">
          <div className="card-header">
            <h2>{t('profile.admin.appearanceTitle')}</h2>
          </div>
          <p className="text-secondary">{t('profile.admin.appearanceLead')}</p>
          <div className="actions-row u-mt-sm">
            <button
              type="button"
              className="btn-secondary"
              onClick={handleAppearanceToggle}
              title={uiTheme === 'dark' ? t('shell.theme.light') : t('shell.theme.dark')}
            >
              {uiTheme === 'dark' ? t('shell.theme.light') : t('shell.theme.dark')}
            </button>
          </div>
        </div>
      )}

      {(error || loadError) && <div className="alert alert-error">{error || loadError}</div>}

      {(showAll || section === 'workspace') && (
        <div className="card">
          <div className="card-header">
            <h2>Configuración del espacio</h2>
          </div>

          {loading && <div className="spinner" aria-label="Cargando" />}

          {!loading && settings && form && (
            <form onSubmit={(e) => void onSubmit(e)} className="admin-settings-form">
              <div className="admin-settings-toolbar">
                <button type="submit" className="btn-primary" disabled={saving}>
                  {saving ? 'Guardando…' : 'Guardar cambios'}
                </button>
                <button type="button" className="btn-secondary" onClick={onResetForm} disabled={saving}>
                  Deshacer cambios
                </button>
              </div>

              <section className="admin-settings-section">
                <h3>Monedas e impuestos</h3>
                <p className="admin-settings-hint">
                  La primera moneda es la principal (documentos y totales por defecto). Podés sumar las que uses en
                  operaciones o cotizaciones.
                </p>
                <div className="admin-currencies-list">
                  {form.currencies.map((code, index) => (
                    <div key={index} className="admin-currency-row">
                      <span className="admin-currency-rank" title="Orden">
                        {index === 0 ? 'Principal' : `${index + 1}`}
                      </span>
                      <input
                        type="text"
                        className="admin-currency-input"
                        value={code}
                        onChange={(e) => updateCurrencyRow(index, e.target.value)}
                        placeholder="ARS"
                        maxLength={8}
                        autoCapitalize="characters"
                      />
                      <div className="admin-currency-actions">
                        <button
                          type="button"
                          className="btn-secondary btn-sm"
                          disabled={index === 0}
                          onClick={() => moveCurrencyRow(index, -1)}
                          title="Subir"
                        >
                          ↑
                        </button>
                        <button
                          type="button"
                          className="btn-secondary btn-sm"
                          disabled={index >= form.currencies.length - 1}
                          onClick={() => moveCurrencyRow(index, 1)}
                          title="Bajar"
                        >
                          ↓
                        </button>
                        <button
                          type="button"
                          className="btn-danger btn-sm"
                          disabled={form.currencies.length <= 1}
                          onClick={() => removeCurrencyRow(index)}
                          title="Quitar"
                        >
                          Quitar
                        </button>
                      </div>
                    </div>
                  ))}
                  <button type="button" className="btn-secondary btn-sm admin-currency-add" onClick={addCurrencyRow}>
                    + Agregar moneda
                  </button>
                </div>
                <div className="admin-settings-grid u-mt-md">
                  <div className="form-group">
                    <label>IVA / impuesto (%)</label>
                    <input
                      type="number"
                      min={0}
                      step="0.01"
                      value={form.tax_rate}
                      onChange={(e) => updateField('tax_rate', e.target.value)}
                    />
                  </div>
                  <div className="form-group admin-checkbox-row">
                    <input
                      id="allow_negative_stock"
                      type="checkbox"
                      checked={form.allow_negative_stock}
                      onChange={(e) => updateField('allow_negative_stock', e.target.checked)}
                    />
                    <label htmlFor="allow_negative_stock">Permitir stock negativo</label>
                  </div>
                </div>
              </section>

              <section className="admin-settings-section">
                <h3>Prefijos y correlativos</h3>
                <p className="admin-settings-hint">
                  Los próximos números los asigna el sistema al crear documentos; no se pueden editar aquí.
                </p>
                <div className="admin-settings-grid">
                  <div className="form-group">
                    <label>Prefijo presupuestos</label>
                    <input
                      type="text"
                      value={form.quote_prefix}
                      onChange={(e) => updateField('quote_prefix', e.target.value)}
                    />
                  </div>
                  <div className="form-group">
                    <label>Próximo presupuesto</label>
                    <input type="text" value={String(settings.next_quote_number)} readOnly className="input-readonly" />
                  </div>
                  <div className="form-group">
                    <label>Prefijo ventas</label>
                    <input
                      type="text"
                      value={form.sale_prefix}
                      onChange={(e) => updateField('sale_prefix', e.target.value)}
                    />
                  </div>
                  <div className="form-group">
                    <label>Próxima venta</label>
                    <input type="text" value={String(settings.next_sale_number)} readOnly className="input-readonly" />
                  </div>
                  <div className="form-group">
                    <label>Prefijo compras</label>
                    <input
                      type="text"
                      value={form.purchase_prefix}
                      onChange={(e) => updateField('purchase_prefix', e.target.value)}
                    />
                  </div>
                  <div className="form-group">
                    <label>Próxima compra</label>
                    <input
                      type="text"
                      value={String(settings.next_purchase_number)}
                      readOnly
                      className="input-readonly"
                    />
                  </div>
                  <div className="form-group">
                    <label>Prefijo devoluciones</label>
                    <input
                      type="text"
                      value={form.return_prefix}
                      onChange={(e) => updateField('return_prefix', e.target.value)}
                    />
                  </div>
                  <div className="form-group">
                    <label>Próxima devolución</label>
                    <input
                      type="text"
                      value={String(settings.next_return_number)}
                      readOnly
                      className="input-readonly"
                    />
                  </div>
                  <div className="form-group">
                    <label>Prefijo notas de crédito</label>
                    <input
                      type="text"
                      value={form.credit_note_prefix}
                      onChange={(e) => updateField('credit_note_prefix', e.target.value)}
                    />
                  </div>
                  <div className="form-group">
                    <label>Próxima nota de crédito</label>
                    <input
                      type="text"
                      value={String(settings.next_credit_note_number)}
                      readOnly
                      className="input-readonly"
                    />
                  </div>
                </div>
              </section>

              <section className="admin-settings-section">
                <h3>Datos del negocio</h3>
                <div className="admin-settings-grid">
                  <div className="form-group grow">
                    <label>Razón social / nombre</label>
                    <input
                      type="text"
                      value={form.business_name}
                      onChange={(e) => updateField('business_name', e.target.value)}
                    />
                  </div>
                  <div className="form-group">
                    <label>CUIT / ID fiscal</label>
                    <input
                      type="text"
                      value={form.business_tax_id}
                      onChange={(e) => updateField('business_tax_id', e.target.value)}
                    />
                  </div>
                  <div className="form-group full-width">
                    <label>Dirección</label>
                    <input
                      type="text"
                      value={form.business_address}
                      onChange={(e) => updateField('business_address', e.target.value)}
                    />
                  </div>
                  <div className="form-group">
                    <label>Teléfono</label>
                    <input
                      type="text"
                      value={form.business_phone}
                      onChange={(e) => updateField('business_phone', e.target.value)}
                    />
                  </div>
                  <div className="form-group grow">
                    <label>Email</label>
                    <input
                      type="email"
                      value={form.business_email}
                      onChange={(e) => updateField('business_email', e.target.value)}
                    />
                  </div>
                  <div className="form-group">
                    <label>Vertical</label>
                    <select value={form.vertical} onChange={(e) => updateField('vertical', e.target.value)}>
                      <option value="">Sin definir</option>
                      <option value="none">Solo comercial</option>
                      <option value="professionals">Profesionales</option>
                      <option value="workshops">Talleres</option>
                      <option value="beauty">Belleza</option>
                      <option value="restaurants">Restaurantes</option>
                    </select>
                  </div>
                </div>
              </section>

              <section className="admin-settings-section">
                <h3>Agenda</h3>
                <div className="admin-settings-grid">
                  <div className="form-group admin-checkbox-row">
                    <input
                      id="scheduling_enabled"
                      type="checkbox"
                      checked={form.scheduling_enabled}
                      onChange={(e) => updateField('scheduling_enabled', e.target.checked)}
                    />
                    <label htmlFor="scheduling_enabled">Agenda habilitada</label>
                  </div>
                  <div className="form-group">
                    <label>Etiqueta (ej. Turno, Clase)</label>
                    <input
                      type="text"
                      value={form.scheduling_label}
                      onChange={(e) => updateField('scheduling_label', e.target.value)}
                    />
                  </div>
                  <div className="form-group">
                    <label>Recordatorio (horas antes)</label>
                    <input
                      type="number"
                      min={0}
                      value={form.scheduling_reminder_hours}
                      onChange={(e) => updateField('scheduling_reminder_hours', e.target.value)}
                    />
                  </div>
                </div>
              </section>

              <section className="admin-settings-section">
                <h3>Cotización</h3>
                <div className="admin-settings-grid">
                  <div className="form-group">
                    <label>Tipo de cotización por defecto</label>
                    <input
                      type="text"
                      value={form.default_rate_type}
                      onChange={(e) => updateField('default_rate_type', e.target.value)}
                      placeholder="blue, oficial…"
                    />
                  </div>
                  <div className="form-group admin-checkbox-row">
                    <input
                      id="auto_fetch_rates"
                      type="checkbox"
                      checked={form.auto_fetch_rates}
                      onChange={(e) => updateField('auto_fetch_rates', e.target.checked)}
                    />
                    <label htmlFor="auto_fetch_rates">Obtener cotizaciones automáticamente</label>
                  </div>
                  <div className="form-group admin-checkbox-row">
                    <input
                      id="show_dual_prices"
                      type="checkbox"
                      checked={form.show_dual_prices}
                      onChange={(e) => updateField('show_dual_prices', e.target.checked)}
                    />
                    <label htmlFor="show_dual_prices">Mostrar precios duales</label>
                  </div>
                </div>
              </section>

              <section className="admin-settings-section">
                <h3>Banco y PDF</h3>
                <div className="admin-settings-grid">
                  <div className="form-group">
                    <label>Titular</label>
                    <input
                      type="text"
                      value={form.bank_holder}
                      onChange={(e) => updateField('bank_holder', e.target.value)}
                    />
                  </div>
                  <div className="form-group">
                    <label>CBU</label>
                    <input
                      type="text"
                      value={form.bank_cbu}
                      onChange={(e) => updateField('bank_cbu', e.target.value)}
                    />
                  </div>
                  <div className="form-group">
                    <label>Alias</label>
                    <input
                      type="text"
                      value={form.bank_alias}
                      onChange={(e) => updateField('bank_alias', e.target.value)}
                    />
                  </div>
                  <div className="form-group grow">
                    <label>Banco</label>
                    <input
                      type="text"
                      value={form.bank_name}
                      onChange={(e) => updateField('bank_name', e.target.value)}
                    />
                  </div>
                  <div className="form-group admin-checkbox-row">
                    <input
                      id="show_qr_in_pdf"
                      type="checkbox"
                      checked={form.show_qr_in_pdf}
                      onChange={(e) => updateField('show_qr_in_pdf', e.target.checked)}
                    />
                    <label htmlFor="show_qr_in_pdf">Mostrar QR en PDF</label>
                  </div>
                </div>
              </section>

              <div className="admin-settings-toolbar admin-settings-toolbar-bottom">
                <button type="submit" className="btn-primary" disabled={saving}>
                  {saving ? 'Guardando…' : 'Guardar cambios'}
                </button>
                <button type="button" className="btn-secondary" onClick={onResetForm} disabled={saving}>
                  Deshacer cambios
                </button>
              </div>
            </form>
          )}
        </div>
      )}

      {(showAll || section === 'rbac') && isConsoleAdmin && sessionOrgId ? (
        <AdminRbacSection orgId={sessionOrgId} />
      ) : null}

      {(showAll || section === 'audit') && (
        <div className="card">
          <div className="card-header admin-card-header--wrap">
            <h2>Registro de auditoría</h2>
            <div className="admin-audit-header-actions">
              <span className="badge badge-neutral">{activity.length} eventos</span>
              <button
                type="button"
                className="btn-sm btn-secondary"
                disabled={auditExportBusy}
                onClick={() => void handleAuditExportCsv()}
              >
                {auditExportBusy ? 'Descargando…' : 'Descargar CSV'}
              </button>
            </div>
          </div>
          {activity.length === 0 ? (
            <div className="empty-state">
              <p>Sin eventos registrados</p>
            </div>
          ) : (
            <div className="admin-activity-wrap">
              <table className="admin-activity-table">
                <thead>
                  <tr>
                    <th>Fecha</th>
                    <th>Acción</th>
                    <th>Recurso</th>
                    <th>ID</th>
                    <th>Actor</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredActivity.slice(0, 50).map((row) => (
                    <tr key={row.id}>
                      <td>{formatDateTime(row.created_at)}</td>
                      <td>
                        <code className="admin-code">{row.action}</code>
                      </td>
                      <td>
                        <code className="admin-code">{row.resource_type}</code>
                      </td>
                      <td className="admin-activity-id">{row.resource_id ?? '—'}</td>
                      <td>{row.actor ?? '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
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
