import { FormEvent, useCallback, useEffect, useState } from 'react';
import { getAuditEntries, getTenantSettings, updateTenantSettings } from '../lib/api';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import type { AuditEntry, TenantSettings, TenantSettingsUpdatePayload } from '../lib/types';

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
  const reminder = Number(f.appointment_reminder_hours);
  if (!Number.isFinite(reminder) || reminder < 0) {
    return { error: 'Las horas de recordatorio deben ser un número ≥ 0.' };
  }

  return {
    currency: f.currency.trim(),
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
    wa_quote_template: f.wa_quote_template,
    wa_receipt_template: f.wa_receipt_template,
    wa_default_country_code: f.wa_default_country_code.trim(),
    appointments_enabled: f.appointments_enabled,
    appointment_label: f.appointment_label.trim(),
    appointment_reminder_hours: reminder,
    secondary_currency: f.secondary_currency.trim(),
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

type TenantFormState = {
  currency: string;
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
  wa_quote_template: string;
  wa_receipt_template: string;
  wa_default_country_code: string;
  appointments_enabled: boolean;
  appointment_label: string;
  appointment_reminder_hours: string;
  secondary_currency: string;
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
    currency: s.currency ?? '',
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
    wa_quote_template: s.wa_quote_template ?? '',
    wa_receipt_template: s.wa_receipt_template ?? '',
    wa_default_country_code: s.wa_default_country_code ?? '',
    appointments_enabled: Boolean(s.appointments_enabled),
    appointment_label: s.appointment_label ?? '',
    appointment_reminder_hours: String(s.appointment_reminder_hours ?? ''),
    secondary_currency: s.secondary_currency ?? '',
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

export function AdminPage() {
  const [settings, setSettings] = useState<TenantSettings | null>(null);
  const [form, setForm] = useState<TenantFormState | null>(null);
  const [activity, setActivity] = useState<AuditEntry[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [tenant, audit] = await Promise.all([getTenantSettings(), getAuditEntries()]);
      setSettings(tenant);
      setForm(settingsToForm(tenant));
      setActivity(audit.items ?? []);
      setError('');
    } catch (err) {
      setError(formatFetchErrorForUser(err, 'No pudimos conectar con el servidor. Verificá tu red.'));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  function updateField<K extends keyof TenantFormState>(key: K, value: TenantFormState[K]): void {
    setForm((prev) => (prev ? { ...prev, [key]: value } : prev));
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
      setSettings(updated);
      setForm(settingsToForm(updated));
    } catch (err) {
      setError(formatFetchErrorForUser(err, 'No pudimos conectar con el servidor. Verificá tu red.'));
    } finally {
      setSaving(false);
    }
  }

  function onResetForm(): void {
    if (settings) setForm(settingsToForm(settings));
    setError('');
  }

  return (
    <>
      <div className="page-header">
        <h1>Administración</h1>
        <p>Configuración del espacio y registro de actividad</p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

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
              <h3>Moneda e impuestos</h3>
              <div className="admin-settings-grid">
                <div className="form-group">
                  <label>Moneda principal</label>
                  <input
                    type="text"
                    value={form.currency}
                    onChange={(e) => updateField('currency', e.target.value)}
                    placeholder="ARS"
                    maxLength={8}
                  />
                </div>
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
                  <input type="text" value={String(settings.next_purchase_number)} readOnly className="input-readonly" />
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
                  <input type="text" value={String(settings.next_return_number)} readOnly className="input-readonly" />
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
              </div>
            </section>

            <section className="admin-settings-section">
              <h3>Turnos / citas</h3>
              <div className="admin-settings-grid">
                <div className="form-group admin-checkbox-row">
                  <input
                    id="appointments_enabled"
                    type="checkbox"
                    checked={form.appointments_enabled}
                    onChange={(e) => updateField('appointments_enabled', e.target.checked)}
                  />
                  <label htmlFor="appointments_enabled">Turnos habilitados</label>
                </div>
                <div className="form-group">
                  <label>Etiqueta (ej. Turno, Clase)</label>
                  <input
                    type="text"
                    value={form.appointment_label}
                    onChange={(e) => updateField('appointment_label', e.target.value)}
                  />
                </div>
                <div className="form-group">
                  <label>Recordatorio (horas antes)</label>
                  <input
                    type="number"
                    min={0}
                    value={form.appointment_reminder_hours}
                    onChange={(e) => updateField('appointment_reminder_hours', e.target.value)}
                  />
                </div>
              </div>
            </section>

            <section className="admin-settings-section">
              <h3>Moneda secundaria y cotización</h3>
              <div className="admin-settings-grid">
                <div className="form-group">
                  <label>Moneda secundaria</label>
                  <input
                    type="text"
                    value={form.secondary_currency}
                    onChange={(e) => updateField('secondary_currency', e.target.value)}
                    placeholder="USD"
                    maxLength={8}
                  />
                </div>
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
                  <input type="text" value={form.bank_holder} onChange={(e) => updateField('bank_holder', e.target.value)} />
                </div>
                <div className="form-group">
                  <label>CBU</label>
                  <input type="text" value={form.bank_cbu} onChange={(e) => updateField('bank_cbu', e.target.value)} />
                </div>
                <div className="form-group">
                  <label>Alias</label>
                  <input type="text" value={form.bank_alias} onChange={(e) => updateField('bank_alias', e.target.value)} />
                </div>
                <div className="form-group grow">
                  <label>Banco</label>
                  <input type="text" value={form.bank_name} onChange={(e) => updateField('bank_name', e.target.value)} />
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

            <section className="admin-settings-section">
              <h3>WhatsApp (plantillas)</h3>
              <p className="admin-settings-hint">
                Podés usar variables entre llaves según las que soporte el backend (ej. nombre de cliente, total).
              </p>
              <div className="admin-settings-grid">
                <div className="form-group">
                  <label>Código país por defecto</label>
                  <input
                    type="text"
                    value={form.wa_default_country_code}
                    onChange={(e) => updateField('wa_default_country_code', e.target.value)}
                    placeholder="54"
                  />
                </div>
                <div className="form-group full-width">
                  <label>Plantilla mensaje presupuesto</label>
                  <textarea
                    className="admin-textarea"
                    rows={3}
                    value={form.wa_quote_template}
                    onChange={(e) => updateField('wa_quote_template', e.target.value)}
                  />
                </div>
                <div className="form-group full-width">
                  <label>Plantilla comprobante / recibo</label>
                  <textarea
                    className="admin-textarea"
                    rows={3}
                    value={form.wa_receipt_template}
                    onChange={(e) => updateField('wa_receipt_template', e.target.value)}
                  />
                </div>
                <div className="form-group full-width">
                  <label>Plantilla pago</label>
                  <textarea
                    className="admin-textarea"
                    rows={3}
                    value={form.wa_payment_template}
                    onChange={(e) => updateField('wa_payment_template', e.target.value)}
                  />
                </div>
                <div className="form-group full-width">
                  <label>Plantilla link de pago</label>
                  <textarea
                    className="admin-textarea"
                    rows={3}
                    value={form.wa_payment_link_template}
                    onChange={(e) => updateField('wa_payment_link_template', e.target.value)}
                  />
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

      <div className="card">
        <div className="card-header">
          <h2>Registro de auditoría</h2>
          <span className="badge badge-neutral">{activity.length} eventos</span>
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
                {activity.slice(0, 50).map((row) => (
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
    </>
  );
}
