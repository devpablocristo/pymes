const BRANCH_STORAGE_PREFIX = 'pymes-ui:branch-selection:';
const ACTIVE_BRANCH_STORAGE_KEY = 'pymes-ui:branch-selection:active';

function normalizeBranchId(branchId: string | null | undefined): string | null {
  return branchId?.trim() || null;
}

export function storageKeyForOrg(orgId: string): string {
  return `${BRANCH_STORAGE_PREFIX}${orgId}`;
}

export function readStoredBranchId(orgId: string): string | null {
  try {
    return normalizeBranchId(window.localStorage.getItem(storageKeyForOrg(orgId)));
  } catch {
    return null;
  }
}

export function writeStoredBranchId(orgId: string, branchId: string | null) {
  try {
    const normalized = normalizeBranchId(branchId);
    if (normalized) {
      window.localStorage.setItem(storageKeyForOrg(orgId), normalized);
      return;
    }
    window.localStorage.removeItem(storageKeyForOrg(orgId));
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
