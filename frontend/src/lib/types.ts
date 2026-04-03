/** Respuesta de GET/PATCH `/v1/admin/tenant-settings` (pymes-core admin). */
export type TenantSettings = {
  org_id: string;
  plan_code: string;
  hard_limits: Record<string, unknown>;
  billing_status: string;
  stripe_customer_id?: string;
  stripe_subscription_id?: string;
  currency: string;
  /** Monedas habilitadas; la primera es la principal (`currency`). */
  supported_currencies?: string[];
  tax_rate: number;
  quote_prefix: string;
  sale_prefix: string;
  next_quote_number: number;
  next_sale_number: number;
  allow_negative_stock: boolean;
  purchase_prefix: string;
  next_purchase_number: number;
  return_prefix: string;
  credit_note_prefix: string;
  next_return_number: number;
  next_credit_note_number: number;
  business_name: string;
  business_tax_id: string;
  business_address: string;
  business_phone: string;
  business_email: string;
  team_size: string;
  sells: string;
  client_label: string;
  uses_billing: boolean;
  payment_method: string;
  vertical: string;
  onboarding_completed_at?: string | null;
  wa_quote_template: string;
  wa_receipt_template: string;
  wa_default_country_code: string;
  scheduling_enabled: boolean;
  appointments_enabled?: boolean;
  appointment_label: string;
  appointment_reminder_hours: number;
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
  updated_by?: string | null;
  updated_at: string;
};

/** Cuerpo de PATCH/PUT tenant-settings (campos opcionales; el backend hace merge). */
export type TenantSettingsUpdatePayload = {
  plan_code?: string;
  hard_limits?: Record<string, unknown>;
  currency?: string;
  supported_currencies?: string[];
  tax_rate?: number;
  quote_prefix?: string;
  sale_prefix?: string;
  allow_negative_stock?: boolean;
  purchase_prefix?: string;
  return_prefix?: string;
  credit_note_prefix?: string;
  business_name?: string;
  business_tax_id?: string;
  business_address?: string;
  business_phone?: string;
  business_email?: string;
  team_size?: string;
  sells?: string;
  client_label?: string;
  uses_billing?: boolean;
  payment_method?: string;
  vertical?: string;
  onboarding_completed_at?: string;
  wa_quote_template?: string;
  wa_receipt_template?: string;
  wa_default_country_code?: string;
  scheduling_enabled?: boolean;
  appointments_enabled?: boolean;
  appointment_label?: string;
  appointment_reminder_hours?: number;
  default_rate_type?: string;
  auto_fetch_rates?: boolean;
  show_dual_prices?: boolean;
  bank_holder?: string;
  bank_cbu?: string;
  bank_alias?: string;
  bank_name?: string;
  show_qr_in_pdf?: boolean;
  wa_payment_template?: string;
  wa_payment_link_template?: string;
};

/** Entrada de GET `/v1/audit`. */
export type AuditEntry = {
  id: string;
  org_id: string;
  actor?: string;
  actor_type?: string;
  action: string;
  resource_type: string;
  resource_id?: string;
  created_at: string;
};

/** Rol de producto (consola): solo admin | user; el rol del token puede ser owner, admin, viewer, etc. */
export type ProductRole = 'admin' | 'user';

export type BootstrapAuthPayload = {
  org_id: string;
  /** Nombre legible desde `orgs.name` (GET /session); puede faltar si no hay fila o está vacío. */
  org_name?: string | null;
  /** Mismo UUID que `org_id`; nombre alineado con kernel `tenant_id`. */
  tenant_id?: string;
  /** Rol crudo del JWT / API (p. ej. owner, admin, viewer, service). */
  role: string;
  product_role: ProductRole;
  scopes: string[];
  actor: string;
  auth_method: string;
  vertical?: string | null;
  onboarding_completed_at?: string | null;
};

/** Auth del tenant vía GET /v1/session. */
export type SessionResponse = {
  auth: BootstrapAuthPayload;
};

/** Respuesta de `GET /v1/users/me` (perfil SaaS / core). */
export type MeProfileUser = {
  id: string;
  external_id: string;
  email: string;
  name: string;
  /** Enriquecido por Pymes (`users.given_name` / `family_name`). */
  given_name?: string | null;
  family_name?: string | null;
  /** Opcional; columna users.phone. */
  phone?: string | null;
  avatar_url?: string | null;
};

export type MeProfileResponse = {
  org_id: string;
  external_id: string;
  role: string;
  scopes?: string[];
  user?: MeProfileUser | null;
};

export type BillingStatus = {
  org_id: string;
  plan_code: string;
  status: string;
  hard_limits: Record<string, unknown>;
  usage: Record<string, unknown>;
  current_period_end: string;
};

export type APIKeyItem = {
  id: string;
  name: string;
  key_prefix: string;
  scopes: string[];
  created_at: string;
};

export type NotificationPreference = {
  user_id: string;
  notification_type: string;
  channel: string;
  enabled: boolean;
};
