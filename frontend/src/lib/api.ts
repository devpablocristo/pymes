import {
  request,
  requestResponse,
  type RequestOptions,
} from '@devpablocristo/core-authn/http/fetch';
import type { DashboardResponse, DashboardSavePayload } from '../dashboard/types';
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

export async function getSession(): Promise<SessionResponse> {
  return request('/v1/session');
}

export async function getTenantSettings(): Promise<TenantSettings> {
  return request('/v1/admin/tenant-settings');
}

export async function updateTenantSettings(
  payload: TenantSettingsUpdatePayload,
): Promise<TenantSettings> {
  return request('/v1/admin/tenant-settings', { method: 'PATCH', body: payload });
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

export async function getDashboard(context = 'home'): Promise<DashboardResponse> {
  return request(`/v1/dashboard?context=${encodeURIComponent(context)}`);
}

export async function saveDashboard(payload: DashboardSavePayload): Promise<DashboardResponse> {
  return request('/v1/dashboard', { method: 'PUT', body: payload });
}

export async function resetDashboard(context = 'home'): Promise<DashboardResponse> {
  return request(`/v1/dashboard/reset?context=${encodeURIComponent(context)}`, {
    method: 'POST',
    body: {},
  });
}

export async function apiRequest<T = unknown>(path: string, options: RequestOptions = {}): Promise<T> {
  return request<T>(path, options);
}

export async function downloadAPIFile(path: string, options: RequestOptions = {}): Promise<string> {
  const response = await requestResponse(path, options);
  const disposition = response.headers.get('content-disposition') ?? '';
  const match = disposition.match(/filename=\"?([^\";]+)\"?/i);
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
