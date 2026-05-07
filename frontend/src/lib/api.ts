import {
  request as coreRequest,
  requestResponse as coreRequestResponse,
  type RequestOptions,
} from '@devpablocristo/core-authn/http/fetch';
import type {
  APIKeyItem,
  BillingStatus,
  MeProfileResponse,
  NotificationPreference,
  SessionResponse,
  AuditEntry,
  TenantSettings,
  TenantSettingsUpdatePayload,
} from './types';

const TENANT_SLUG_HEADER = 'X-Pymes-Tenant-Slug';
const RESERVED_TENANT_PATHS = new Set(['login', 'signup', 'invite', 'onboarding']);

export type TenantAwareRequestOptions = RequestOptions & {
  tenantSlug?: string | null;
  skipTenantSlug?: boolean;
};

type TenantSlugProvider = () => string | null;

let tenantSlugProvider: TenantSlugProvider | null = null;

export function registerTenantSlugProvider(provider: TenantSlugProvider): () => void {
  tenantSlugProvider = provider;
  return () => {
    if (tenantSlugProvider === provider) {
      tenantSlugProvider = null;
    }
  };
}

function withTenantSlugHeader(options: TenantAwareRequestOptions = {}): RequestOptions {
  const { tenantSlug, skipTenantSlug, ...rest } = options;
  const slug = (
    tenantSlug ??
    (skipTenantSlug ? null : tenantSlugProvider?.() ?? readTenantSlugFromLocation()) ??
    ''
  ).trim();
  if (!slug) {
    return rest;
  }
  return {
    ...rest,
    headers: {
      ...(rest.headers ?? {}),
      [TENANT_SLUG_HEADER]: slug,
    },
  };
}

function readTenantSlugFromLocation(): string | null {
  if (typeof window === 'undefined') {
    return null;
  }
  const segment = window.location.pathname.split('/').find((part) => part.trim() !== '')?.trim() ?? '';
  if (!segment || RESERVED_TENANT_PATHS.has(segment)) {
    return null;
  }
  return segment;
}

async function request<T = unknown>(path: string, options: TenantAwareRequestOptions = {}): Promise<T> {
  return coreRequest<T>(path, withTenantSlugHeader(options));
}

async function requestResponse(path: string, options: TenantAwareRequestOptions = {}): Promise<Response> {
  return coreRequestResponse(path, withTenantSlugHeader(options));
}

function resolveAPIBaseURL(): string {
  const configured = import.meta.env.VITE_API_URL?.trim();
  if (configured) {
    return configured.replace(/\/$/, '');
  }
  if (typeof window === 'undefined') {
    return 'http://localhost:8100';
  }
  return `${window.location.protocol}//${window.location.hostname || 'localhost'}:8100`;
}

async function readSetupKeyError(response: Response): Promise<Error> {
  const text = await response.text().catch(() => response.statusText);
  if (!text) {
    return new Error(response.statusText || `HTTP ${response.status}`);
  }
  try {
    const body = JSON.parse(text) as { error?: string | { message?: string; code?: string }; message?: string };
    if (body.error && typeof body.error === 'object') {
      return new Error(body.error.message || body.error.code || text);
    }
    if (typeof body.error === 'string') {
      return new Error(body.error);
    }
    if (body.message) {
      return new Error(body.message);
    }
  } catch {
    // keep raw text below
  }
  return new Error(text);
}

async function requestWithTenantSetupKey<T = unknown>(
  path: string,
  apiKey: string,
  tenantSlug: string,
  options: Pick<TenantAwareRequestOptions, 'method' | 'body'> = {},
): Promise<T> {
  const response = await fetch(`${resolveAPIBaseURL()}${path}`, {
    method: options.method ?? 'GET',
    headers: {
      'Content-Type': 'application/json',
      'X-API-KEY': apiKey,
      [TENANT_SLUG_HEADER]: tenantSlug,
    },
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });
  if (!response.ok) {
    throw await readSetupKeyError(response);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  const contentType = response.headers.get('content-type') ?? '';
  if (contentType.includes('application/json')) {
    return (await response.json()) as T;
  }
  return (await response.text()) as T;
}

function normalizeTenantSettings(settings: TenantSettings): TenantSettings {
  return {
    ...settings,
    scheduling_enabled: Boolean(settings.scheduling_enabled),
  };
}

export async function getSession(options: TenantAwareRequestOptions = {}): Promise<SessionResponse> {
  return request('/v1/session', options);
}

export async function getTenantSettings(options: TenantAwareRequestOptions = {}): Promise<TenantSettings> {
  const response = await request<TenantSettings>('/v1/admin/tenant-settings', options);
  return normalizeTenantSettings(response);
}

export async function updateTenantSettings(
  payload: TenantSettingsUpdatePayload,
  options: TenantAwareRequestOptions = {},
): Promise<TenantSettings> {
  const response = await request<TenantSettings>('/v1/admin/tenant-settings', {
    ...options,
    method: 'PATCH',
    body: payload,
  });
  return normalizeTenantSettings(response);
}

export async function updateTenantSettingsWithSetupKey(
  payload: TenantSettingsUpdatePayload,
  apiKey: string,
  tenantSlug: string,
): Promise<TenantSettings> {
  const response = await requestWithTenantSetupKey<TenantSettings>(
    '/v1/admin/tenant-settings',
    apiKey,
    tenantSlug,
    { method: 'PATCH', body: payload },
  );
  return normalizeTenantSettings(response);
}

export type SchedulingBranchSummary = {
  id: string;
  code: string;
  name: string;
  timezone: string;
  address?: string;
  active: boolean;
};

export async function listSchedulingBranches(options: TenantAwareRequestOptions = {}): Promise<{ items: SchedulingBranchSummary[] }> {
  return request('/v1/scheduling/branches', options);
}

export async function listSchedulingBranchesWithSetupKey(
  apiKey: string,
  tenantSlug: string,
): Promise<{ items: SchedulingBranchSummary[] }> {
  return requestWithTenantSetupKey('/v1/scheduling/branches', apiKey, tenantSlug);
}

export async function createSchedulingBranch(payload: {
  code: string;
  name: string;
  timezone: string;
  address?: string;
  active?: boolean;
}, options: TenantAwareRequestOptions = {}): Promise<SchedulingBranchSummary> {
  return request('/v1/scheduling/branches', { ...options, method: 'POST', body: payload });
}

export async function createSchedulingBranchWithSetupKey(payload: {
  code: string;
  name: string;
  timezone: string;
  address?: string;
  active?: boolean;
}, apiKey: string, tenantSlug: string): Promise<SchedulingBranchSummary> {
  return requestWithTenantSetupKey('/v1/scheduling/branches', apiKey, tenantSlug, {
    method: 'POST',
    body: payload,
  });
}

export async function getBillingStatus(): Promise<BillingStatus> {
  return request('/v1/billing/status');
}

export type TenantSummary = {
  id: string;
  slug?: string;
  name: string;
  clerk_org_id?: string;
  role: 'owner' | 'admin' | 'member' | string;
};

export async function listTenants(): Promise<{ items: TenantSummary[] }> {
  return request('/v1/tenants');
}

export async function createTenant(payload: {
  name: string;
  slug?: string;
}): Promise<{ tenant_id: string; clerk_org_id: string; slug?: string; raw_key?: string; key?: APIKeyItem }> {
  return request('/v1/tenants', { method: 'POST', body: payload });
}

export async function createPortal(payload: { return_url: string }): Promise<{ portal_url: string }> {
  return request('/v1/billing/portal', { method: 'POST', body: payload });
}

export async function getAPIKeys(tenantID: string): Promise<{ items: APIKeyItem[] }> {
  return request(`/v1/tenants/${tenantID}/api-keys`);
}

export async function createAPIKey(
  tenantID: string,
  payload: { name: string; scopes: string[] },
): Promise<{ key: APIKeyItem; raw_key: string }> {
  return request(`/v1/tenants/${tenantID}/api-keys`, { method: 'POST', body: payload });
}

export async function rotateAPIKey(tenantID: string, keyID: string): Promise<{ key: APIKeyItem; raw_key: string }> {
  return request(`/v1/tenants/${tenantID}/api-keys/${keyID}/rotate`, { method: 'POST', body: {} });
}

export async function deleteAPIKey(tenantID: string, keyID: string): Promise<void> {
  await request(`/v1/tenants/${tenantID}/api-keys/${keyID}`, { method: 'DELETE' });
}

export type InAppNotificationItem = {
  id: string;
  title: string;
  body: string;
  kind: string;
  entity_type: string;
  entity_id: string;
  chat_context: Record<string, unknown>;
  read_at: string | null;
  created_at: string;
};

export async function listInAppNotifications(): Promise<{
  items: InAppNotificationItem[];
  unread_count: number;
}> {
  return request('/v1/in-app-notifications');
}

export async function getNotificationsSummary(): Promise<{ unread_count: number }> {
  return request('/v1/in-app-notifications/summary');
}

export async function markInAppNotificationRead(id: string): Promise<{ id: string; read_at: string }> {
  return request(`/v1/in-app-notifications/${id}`, { method: 'PATCH', body: { read: true } });
}

export async function getNotificationPreferences(): Promise<{ items: NotificationPreference[] }> {
  return request('/v1/notifications/preferences');
}

export async function updateNotificationPreference(payload: {
  notification_type: string;
  channel: string;
  enabled: boolean;
}): Promise<NotificationPreference> {
  return request('/v1/notifications/preferences', { method: 'PUT', body: payload });
}

export async function getAuditEntries(): Promise<{ items: AuditEntry[] }> {
  return request('/v1/audit');
}

export async function downloadAuditExportCsv(): Promise<string> {
  return downloadAPIFile('/v1/audit/export?format=csv');
}

export type TenantMemberRow = {
  id: string;
  tenant_id?: string;
  user_id: string;
  role?: string;
  status?: string;
  joined_at?: string;
  user?: { id?: string; email?: string; name?: string };
};

export async function listTenantMembers(tenantId: string): Promise<{ items: TenantMemberRow[] }> {
  return request(`/v1/tenants/${tenantId}/members`);
}

export type TenantInvitation = {
  id: string;
  tenant_id: string;
  email: string;
  role: string;
  status: 'pending' | 'accepted' | 'revoked' | 'expired';
  clerk_invitation_id?: string;
  invited_by_user_id: string;
  accepted_by_user_id?: string;
  expires_at: string;
  accepted_at?: string;
  revoked_at?: string;
  created_at: string;
  updated_at: string;
};

export async function listTenantInvites(tenantId: string): Promise<{ items: TenantInvitation[] }> {
  return request(`/v1/tenants/${tenantId}/invites`);
}

export async function createTenantInvite(
  tenantId: string,
  payload: { email: string; role: string },
): Promise<{ invite: TenantInvitation }> {
  return request(`/v1/tenants/${tenantId}/invites`, { method: 'POST', body: payload });
}

export async function revokeTenantInvite(inviteId: string): Promise<{ invite: TenantInvitation }> {
  return request(`/v1/tenant-invites/${inviteId}/revoke`, { method: 'POST', body: {} });
}

export async function resendTenantInvite(inviteId: string): Promise<{ invite: TenantInvitation }> {
  return request(`/v1/tenant-invites/${inviteId}/resend`, { method: 'POST', body: {} });
}

export async function acceptTenantInvite(token: string): Promise<{ invite: TenantInvitation; clerk_org_id: string }> {
  return request('/v1/tenant-invites/accept', { method: 'POST', body: { token } });
}

export type RbacRoleSummary = {
  id: string;
  name: string;
  description?: string;
};

export async function listRbacRoles(): Promise<{ items: RbacRoleSummary[] }> {
  return request('/v1/roles');
}

export async function assignRbacRole(roleId: string, userId: string): Promise<void> {
  await request(`/v1/roles/${roleId}/assign/${userId}`, { method: 'POST', body: {} });
}

export async function removeRbacRoleAssignment(roleId: string, userId: string): Promise<void> {
  await request(`/v1/roles/${roleId}/assign/${userId}`, { method: 'DELETE' });
}

export async function getUserEffectivePermissions(userId: string): Promise<{ permissions: Record<string, string[]> }> {
  return request(`/v1/users/${userId}/permissions`);
}

export type SalePaymentRow = {
  id: string;
  tenant_id?: string;
  reference_type?: string;
  reference_id?: string;
  method: string;
  amount: number;
  notes?: string;
  received_at: string;
  is_favorite?: boolean;
  tags?: string[];
  archived_at?: string | null;
  created_by?: string;
  created_at?: string;
};

export async function listSalePayments(saleId: string): Promise<{ items: SalePaymentRow[] }> {
  return request(`/v1/sales/${saleId}/payments`);
}

export async function createSalePayment(
  saleId: string,
  body: { method: string; amount: number; notes?: string; received_at?: string },
): Promise<SalePaymentRow> {
  return request(`/v1/sales/${saleId}/payments`, { method: 'POST', body });
}

export async function getMe(): Promise<MeProfileResponse> {
  return request('/v1/users/me');
}

export async function patchMeProfile(payload: {
  name?: string;
  given_name?: string;
  family_name?: string;
  phone?: string;
}): Promise<MeProfileResponse> {
  return request('/v1/users/me/profile', { method: 'PATCH', body: payload });
}

export async function apiRequest<T = unknown>(path: string, options: TenantAwareRequestOptions = {}): Promise<T> {
  return request<T>(path, options);
}

export async function downloadAPIFile(path: string, options: TenantAwareRequestOptions = {}): Promise<string> {
  const response = await requestResponse(path, options);
  const disposition = response.headers.get('content-disposition') ?? '';
  const match = disposition.match(/filename="?([^";]+)"?/i);
  const filename = match?.[1] ?? `download-${Date.now()}`;
  const blob = await response.blob();
  const url = window.URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  window.URL.revokeObjectURL(url);
  return filename;
}

// ── Customer Messaging Campaigns ──

export type CustomerMessagingCampaign = {
  id: string;
  name: string;
  template_name: string;
  template_language: string;
  template_params: string[];
  tag_filter: string;
  status: string;
  total_recipients: number;
  sent_count: number;
  delivered_count: number;
  read_count: number;
  failed_count: number;
  scheduled_at?: string;
  started_at?: string;
  completed_at?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
};

export type CustomerMessagingCampaignRecipient = {
  id: string;
  party_id: string;
  phone: string;
  party_name: string;
  status: string;
  wa_message_id?: string;
  error_message?: string;
  sent_at?: string;
  delivered_at?: string;
  read_at?: string;
};

export async function listCustomerMessagingCampaigns(): Promise<{ items: CustomerMessagingCampaign[] }> {
  return apiRequest('/v1/customer-messaging/campaigns');
}

export async function getCustomerMessagingCampaign(
  id: string,
): Promise<CustomerMessagingCampaign & { recipients: CustomerMessagingCampaignRecipient[] }> {
  return apiRequest(`/v1/customer-messaging/campaigns/${id}`);
}

export async function createCustomerMessagingCampaign(data: {
  name: string;
  template_name: string;
  template_language?: string;
  template_params?: string[];
  tag_filter?: string;
}): Promise<CustomerMessagingCampaign> {
  return apiRequest('/v1/customer-messaging/campaigns', { method: 'POST', body: data });
}

export async function sendCustomerMessagingCampaign(id: string): Promise<{ status: string }> {
  return apiRequest(`/v1/customer-messaging/campaigns/${id}/send`, { method: 'POST' });
}

// ── Customer Messaging Conversations ──

export type CustomerMessagingConversation = {
  id: string;
  party_id: string;
  phone: string;
  party_name: string;
  assigned_to: string;
  status: string;
  last_message_at?: string;
  last_message_preview: string;
  unread_count: number;
  created_at: string;
  updated_at: string;
};

export async function listCustomerMessagingConversations(params?: {
  assigned_to?: string;
  status?: string;
}): Promise<{ items: CustomerMessagingConversation[] }> {
  const q = new URLSearchParams();
  if (params?.assigned_to) q.set('assigned_to', params.assigned_to);
  if (params?.status) q.set('status', params.status);
  const suffix = q.toString() ? `?${q.toString()}` : '';
  return apiRequest(`/v1/customer-messaging/conversations${suffix}`);
}

export async function assignCustomerMessagingConversation(id: string, assignedTo: string): Promise<{ status: string }> {
  return apiRequest(`/v1/customer-messaging/conversations/${id}/assign`, { method: 'POST', body: { assigned_to: assignedTo } });
}

export async function markCustomerMessagingConversationRead(id: string): Promise<{ status: string }> {
  return apiRequest(`/v1/customer-messaging/conversations/${id}/read`, { method: 'POST' });
}

export async function resolveCustomerMessagingConversation(id: string): Promise<{ status: string }> {
  return apiRequest(`/v1/customer-messaging/conversations/${id}/resolve`, { method: 'POST' });
}
