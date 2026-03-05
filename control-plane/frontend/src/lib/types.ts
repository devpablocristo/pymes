export type TenantSettings = {
  org_id: string;
  plan_code: string;
  hard_limits: Record<string, unknown>;
  updated_at: string;
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
