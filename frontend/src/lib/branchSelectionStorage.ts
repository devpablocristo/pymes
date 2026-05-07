const BRANCH_STORAGE_PREFIX = 'pymes-ui:branch-selection:';
const ACTIVE_BRANCH_STORAGE_KEY = 'pymes-ui:branch-selection:active';

function normalizeBranchId(branchId: string | null | undefined): string | null {
  return branchId?.trim() || null;
}

export function storageKeyForTenant(tenantId: string): string {
  return `${BRANCH_STORAGE_PREFIX}${tenantId}`;
}

export function readStoredBranchId(tenantId: string): string | null {
  try {
    return normalizeBranchId(window.localStorage.getItem(storageKeyForTenant(tenantId)));
  } catch {
    return null;
  }
}

export function writeStoredBranchId(tenantId: string, branchId: string | null) {
  try {
    const normalized = normalizeBranchId(branchId);
    if (normalized) {
      window.localStorage.setItem(storageKeyForTenant(tenantId), normalized);
      return;
    }
    window.localStorage.removeItem(storageKeyForTenant(tenantId));
  } catch {
    // localStorage puede no estar disponible; no bloquear la UI.
  }
}

export function readActiveBranchId(): string | null {
  try {
    return normalizeBranchId(window.localStorage.getItem(ACTIVE_BRANCH_STORAGE_KEY));
  } catch {
    return null;
  }
}

export function writeActiveBranchId(branchId: string | null) {
  try {
    const normalized = normalizeBranchId(branchId);
    if (normalized) {
      window.localStorage.setItem(ACTIVE_BRANCH_STORAGE_KEY, normalized);
      return;
    }
    window.localStorage.removeItem(ACTIVE_BRANCH_STORAGE_KEY);
  } catch {
    // localStorage puede no estar disponible; no bloquear la UI.
  }
}
