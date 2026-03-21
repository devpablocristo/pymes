import {
  request,
  requestResponse,
  type RequestOptions,
} from '@devpablocristo/core-authn/http/fetch';
import type {
  DashboardResponse,
  DashboardSavePayload,
  DashboardWidgetCatalogResponse,
} from '../dashboard/types';
import type { APIKeyItem, BillingStatus, NotificationPreference, TenantSettings } from './types';

export async function getAdminBootstrap(): Promise<{ settings: TenantSettings }> {
  return request('/v1/admin/bootstrap');
}

export async function getTenantSettings(): Promise<TenantSettings> {
  return request('/v1/admin/tenant-settings');
}

export async function updateTenantSettings(payload: {
  plan_code: string;
  hard_limits?: Record<string, unknown>;
}): Promise<TenantSettings> {
  return request('/v1/admin/tenant-settings', { method: 'PUT', body: payload });
}

export async function getBillingStatus(): Promise<BillingStatus> {
  return request('/v1/billing/status');
}

export async function createCheckout(payload: {
  plan_code: string;
  success_url: string;
  cancel_url: string;
}): Promise<{ checkout_url: string }> {
  return request('/v1/billing/checkout', { method: 'POST', body: payload });
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

export async function getAuditEntries(): Promise<{ items: unknown[] }> {
  return request('/v1/audit');
}

export async function getMe(): Promise<Record<string, unknown>> {
  return request('/v1/users/me');
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

export async function getDashboardWidgets(context = 'home'): Promise<DashboardWidgetCatalogResponse> {
  return request(`/v1/dashboard/widgets?context=${encodeURIComponent(context)}`);
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
