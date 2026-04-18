import {
  useEffect,
  useMemo,
  useState,
  type PropsWithChildren,
} from 'react';
import { useQuery } from '@tanstack/react-query';
import { createSchedulingClient, type Branch } from '@devpablocristo/modules-scheduling/next';
import { apiRequest, getSession } from './api';
import {
  readStoredBranchId,
  writeActiveBranchId,
  writeStoredBranchId,
} from './branchSelectionStorage';
import { BranchContext, type BranchContextValue } from './branchSelectionContext';
import { queryKeys } from './queryKeys';

const schedulingClient = createSchedulingClient(apiRequest);
const EMPTY_BRANCHES: Branch[] = [];

export function BranchProvider({ children }: PropsWithChildren) {
  const sessionQuery = useQuery({
    queryKey: queryKeys.session.current,
    queryFn: getSession,
    staleTime: 60_000,
    retry: 1,
  });

  const orgId = sessionQuery.data?.auth.org_id ?? null;

  const branchesQuery = useQuery<Branch[]>({
    queryKey: queryKeys.scheduling.branches,
    queryFn: () => schedulingClient.listBranches(),
    enabled: Boolean(orgId),
    staleTime: 60_000,
    retry: 1,
  });

  const branches = branchesQuery.data ?? EMPTY_BRANCHES;
  const availableBranches = useMemo(() => {
    const active = branches.filter((branch) => branch.active);
    return active.length > 0 ? active : branches;
  }, [branches]);

  const [storedBranchId, setStoredBranchId] = useState<string | null>(null);
  const [selectionHydrated, setSelectionHydrated] = useState(false);

  useEffect(() => {
    if (!orgId) {
      setStoredBranchId(null);
      setSelectionHydrated(false);
      writeActiveBranchId(null);
      return;
    }
    setStoredBranchId(readStoredBranchId(orgId));
    setSelectionHydrated(true);
  }, [orgId]);

  const selectedBranchId = useMemo(() => {
    if (!selectionHydrated) {
      return null;
    }
    if (availableBranches.length === 0) {
      return null;
    }
    if (storedBranchId && availableBranches.some((branch) => branch.id === storedBranchId)) {
      return storedBranchId;
    }
    return availableBranches[0]?.id ?? null;
  }, [availableBranches, selectionHydrated, storedBranchId]);

  useEffect(() => {
    if (!orgId || !selectionHydrated) {
      return;
    }
    writeStoredBranchId(orgId, selectedBranchId);
  }, [orgId, selectedBranchId, selectionHydrated]);

  useEffect(() => {
    if (!selectionHydrated) {
      return;
    }
    writeActiveBranchId(selectedBranchId);
  }, [selectedBranchId, selectionHydrated]);

  const selectedBranch = useMemo(
    () => availableBranches.find((branch) => branch.id === selectedBranchId) ?? null,
    [availableBranches, selectedBranchId],
  );

  const value = useMemo<BranchContextValue>(
    () => ({
      orgId,
      branches,
      availableBranches,
      selectedBranchId,
      selectedBranch,
      isLoading:
        sessionQuery.isLoading ||
        branchesQuery.isLoading ||
        (Boolean(orgId) && !selectionHydrated),
      isError: sessionQuery.isError || branchesQuery.isError,
      error: (sessionQuery.error as Error | null) ?? (branchesQuery.error as Error | null) ?? null,
      setSelectedBranchId: setStoredBranchId,
    }),
    [
      availableBranches,
      branches,
      branchesQuery.error,
      branchesQuery.isError,
      branchesQuery.isLoading,
      orgId,
      selectedBranch,
      selectedBranchId,
      selectionHydrated,
      sessionQuery.error,
      sessionQuery.isError,
      sessionQuery.isLoading,
    ],
  );

  return <BranchContext.Provider value={value}>{children}</BranchContext.Provider>;
}
