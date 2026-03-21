export type TenantSettings = {
  org_id: string;
  plan_code: string;
  hard_limits: Record<string, unknown>;
  updated_at: string;
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
};

export type AdminBootstrapResponse = {
  auth: BootstrapAuthPayload;
  settings: TenantSettings;
};

/** Misma forma que `AdminBootstrapResponse.auth` envuelta; para cualquier usuario autenticado. */
export type SessionResponse = {
  auth: BootstrapAuthPayload;
};

/** Respuesta de `GET /v1/users/me` (perfil SaaS / core). */
export type MeProfileUser = {
  id: string;
  external_id: string;
  email: string;
  name: string;
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
