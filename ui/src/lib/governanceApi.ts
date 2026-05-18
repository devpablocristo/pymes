import { apiRequest } from './api';

// --- Types ---

export interface PolicyResponse {
  id: string;
  name: string;
  action_type: string;
  effect: 'allow' | 'deny' | 'require_approval';
  mode: 'enforced' | 'shadow';
  expression: string;
  created_at: string;
  updated_at: string;
}

export interface CreatePolicyRequest {
  name: string;
  action_type: string;
  effect: 'allow' | 'deny' | 'require_approval';
  condition?: string;
  mode?: 'enforced' | 'shadow';
}

export interface UpdatePolicyRequest {
  name?: string;
  effect?: 'allow' | 'deny' | 'require_approval';
  condition?: string;
  mode?: 'enforced' | 'shadow';
}

export interface PolicyListResponse {
  policies: PolicyResponse[];
  total: number;
}

export interface ActionTypeResponse {
  name: string;
  display_name: string;
  risk_class: string;
  category: string;
}

export interface ActionTypeListResponse {
  action_types: ActionTypeResponse[];
}

export interface ApprovalResponse {
  id: string;
  org_id?: string;
  request_id: string;
  action_type: string;
  target_resource: string;
  reason: string;
  risk_level: string;
  status: string;
  ai_summary?: string;
  created_at: string;
  expires_at?: string;
}

export interface ApprovalListResponse {
  approvals: ApprovalResponse[];
  total: number;
}

export interface ConditionTemplate {
  label: string;
  pattern: string;
  param_name: string;
  param_type: string;
  default_value: string;
}

export interface WatcherResponse {
  id: string;
  name: string;
  watcher_type: string;
  config: Record<string, unknown>;
  enabled: boolean;
  last_run_at: string | null;
  last_result: { found: number; proposed: number; executed: number } | null;
}

// --- Policies ---

export async function listPolicies(): Promise<PolicyListResponse> {
  return apiRequest('/v1/governance/policies');
}

export async function createPolicy(req: CreatePolicyRequest): Promise<PolicyResponse> {
  return apiRequest('/v1/governance/policies', { method: 'POST', body: req });
}

export async function updatePolicy(id: string, req: UpdatePolicyRequest): Promise<PolicyResponse> {
  return apiRequest(`/v1/governance/policies/${id}`, { method: 'PATCH', body: req });
}

export async function deletePolicy(id: string): Promise<void> {
  await apiRequest(`/v1/governance/policies/${id}`, { method: 'DELETE' });
}

// --- Action Types ---

export async function listActionTypes(): Promise<ActionTypeListResponse> {
  return apiRequest('/v1/governance/action-types');
}

// --- Approvals ---

export async function listPendingApprovals(): Promise<ApprovalListResponse> {
  return apiRequest('/v1/governance/approvals/pending');
}

export async function approveRequest(id: string, note: string): Promise<void> {
  await apiRequest(`/v1/governance/approvals/${id}/approve`, { method: 'POST', body: { note } });
}

export async function rejectRequest(id: string, note: string): Promise<void> {
  await apiRequest(`/v1/governance/approvals/${id}/reject`, { method: 'POST', body: { note } });
}

// --- Condition Templates ---

export async function getConditionTemplates(actionType: string): Promise<{ templates: ConditionTemplate[] }> {
  return apiRequest(`/v1/governance/condition-templates/${actionType}`);
}

// --- Watchers ---

export async function listWatchers(): Promise<{ watchers: WatcherResponse[] }> {
  return apiRequest('/v1/governance/watchers');
}

export async function updateWatcher(
  id: string,
  config: Record<string, unknown>,
  enabled: boolean,
): Promise<WatcherResponse> {
  return apiRequest(`/v1/governance/watchers/${id}`, { method: 'PATCH', body: { config, enabled } });
}
