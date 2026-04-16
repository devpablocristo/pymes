import { request, requestResponse, type RequestOptions } from '@devpablocristo/core-authn/http/fetch';
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

function normalizeTenantSettings(settings: TenantSettings): TenantSettings {
  return {
    ...settings,
    scheduling_enabled: Boolean(settings.scheduling_enabled),
  };
}

export async function getSession(): Promise<SessionResponse> {
  return request('/v1/session');
}

export async function getTenantSettings(): Promise<TenantSettings> {
  const response = await request<TenantSettings>('/v1/admin/tenant-settings');
  return normalizeTenantSettings(response);
}

export async function updateTenantSettings(payload: TenantSettingsUpdatePayload): Promise<TenantSettings> {
  const response = await request<TenantSettings>('/v1/admin/tenant-settings', { method: 'PATCH', body: payload });
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

export async function listSchedulingBranches(): Promise<{ items: SchedulingBranchSummary[] }> {
  return request('/v1/scheduling/branches');
}

export async function createSchedulingBranch(payload: {
  code: string;
  name: string;
  timezone: string;
  address?: string;
  active?: boolean;
}): Promise<SchedulingBranchSummary> {
  return request('/v1/scheduling/branches', { method: 'POST', body: payload });
}

export async function getBillingStatus(): Promise<BillingStatus> {
  return request('/v1/billing/status');
}

export async function createPortal(payload: { return_url: string }): Promise<{ portal_url: string }> {
  return request('/v1/billing/portal', { method: 'POST', body: payload });
}

export async function getAPIKeys(orgID: string): Promise<{ items: APIKeyItem[] }> {
  return request(`/v1/orgs/${orgID}/api-keys`);
}

export async function createAPIKey(
  orgID: string,
  payload: { name: string; scopes: string[] },
): Promise<{ key: APIKeyItem; raw_key: string }> {
  return request(`/v1/orgs/${orgID}/api-keys`, { method: 'POST', body: payload });
}

export async function rotateAPIKey(orgID: string, keyID: string): Promise<{ key: APIKeyItem; raw_key: string }> {
  return request(`/v1/orgs/${orgID}/api-keys/${keyID}/rotate`, { method: 'POST', body: {} });
}

export async function deleteAPIKey(orgID: string, keyID: string): Promise<void> {
  await request(`/v1/orgs/${orgID}/api-keys/${keyID}`, { method: 'DELETE' });
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

export type OrgMemberRow = {
  id: string;
  org_id?: string;
  user_id: string;
  role?: string;
  joined_at?: string;
  user?: { id?: string; email?: string; name?: string };
};

export async function listOrgMembers(orgId: string): Promise<{ items: OrgMemberRow[] }> {
  return request(`/v1/orgs/${orgId}/members`);
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
  org_id?: string;
  reference_type?: string;
  reference_id?: string;
  method: string;
  amount: number;
  notes?: string;
  received_at: string;
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

export async function apiRequest<T = unknown>(path: string, options: RequestOptions = {}): Promise<T> {
  return request<T>(path, options);
}

export async function downloadAPIFile(path: string, options: RequestOptions = {}): Promise<string> {
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
