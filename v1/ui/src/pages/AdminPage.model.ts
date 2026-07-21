import type { TenantSettings, TenantSettingsUpdatePayload } from '../lib/types';

export function formatDateTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString('es-AR', {
      dateStyle: 'short',
      timeStyle: 'short',
    });
  } catch {
    return iso;
  }
}

export function buildPayload(f: TenantFormState): TenantSettingsUpdatePayload | { error: string } {
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

export function currenciesFromTenant(s: TenantSettings): string[] {
  if (Array.isArray(s.supported_currencies) && s.supported_currencies.length > 0) {
    return s.supported_currencies.map((c) => String(c).trim());
  }
  const cur = (s.currency ?? 'ARS').trim() || 'ARS';
  const sec = (s.secondary_currency ?? '').trim();
  return sec ? [cur, sec] : [cur];
}

export type TenantFormState = {
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

export function settingsToForm(s: TenantSettings): TenantFormState {
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
