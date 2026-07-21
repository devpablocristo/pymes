import { useContext } from 'react';
import { BranchContext, type BranchContextValue } from './branchSelectionContext';

export function useBranchSelection(): BranchContextValue {
  const value = useContext(BranchContext);
  if (!value) {
    throw new Error('useBranchSelection must be used within BranchProvider');
  }
  return value;
}

export function useOptionalBranchSelection(): BranchContextValue | null {
  return useContext(BranchContext);
}
