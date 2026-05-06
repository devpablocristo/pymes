import type { TenantSettings } from '../lib/types';
import type { TenantFormState } from './AdminPage.model';

type AdminWorkspaceSettingsFormProps = {
  settings: TenantSettings;
  form: TenantFormState;
  saving: boolean;
  onSubmit: (event: React.FormEvent) => void;
  onReset: () => void;
  updateField: <K extends keyof TenantFormState>(key: K, value: TenantFormState[K]) => void;
  updateCurrencyRow: (index: number, value: string) => void;
  addCurrencyRow: () => void;
  removeCurrencyRow: (index: number) => void;
  moveCurrencyRow: (index: number, delta: number) => void;
};

export function AdminWorkspaceSettingsForm({
  settings,
  form,
  saving,
  onSubmit,
  onReset,
  updateField,
  updateCurrencyRow,
  addCurrencyRow,
  removeCurrencyRow,
  moveCurrencyRow,
}: AdminWorkspaceSettingsFormProps) {
  return (
    <form onSubmit={onSubmit} className="admin-settings-form">
      <div className="admin-settings-toolbar">
        <button type="submit" className="btn-primary" disabled={saving}>
          {saving ? 'Guardando…' : 'Guardar cambios'}
        </button>
        <button type="button" className="btn-secondary" onClick={onReset} disabled={saving}>
          Deshacer cambios
        </button>
      </div>

      <section className="admin-settings-section">
        <h3>Monedas e impuestos</h3>
        <p className="admin-settings-hint">
          La primera moneda es la principal (documentos y totales por defecto). Podés sumar las que uses en operaciones
          o cotizaciones.
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
            <input type="text" value={form.sale_prefix} onChange={(e) => updateField('sale_prefix', e.target.value)} />
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
            <input type="text" value={String(settings.next_credit_note_number)} readOnly className="input-readonly" />
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

      <div className="admin-settings-toolbar admin-settings-toolbar-bottom">
        <button type="submit" className="btn-primary" disabled={saving}>
          {saving ? 'Guardando…' : 'Guardar cambios'}
        </button>
        <button type="button" className="btn-secondary" onClick={onReset} disabled={saving}>
          Deshacer cambios
        </button>
      </div>
    </form>
  );
}
