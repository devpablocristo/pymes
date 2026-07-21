import { createContext } from 'react';
import type { Branch } from '@devpablocristo/platform-scheduling/next';

export type BranchContextValue = {
  tenantId: string | null;
  branches: Branch[];
  availableBranches: Branch[];
  selectedBranchId: string | null;
  selectedBranch: Branch | null;
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
  setSelectedBranchId: (branchId: string | null) => void;
};

export const BranchContext = createContext<BranchContextValue | null>(null);
