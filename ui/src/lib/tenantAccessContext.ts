import { createContext, useContext } from 'react';
import type { SessionResponse, TenantSettings } from './types';

export type TenantAccess = {
  tenantId: string;
  tenantSlug: string;
  tenantName: string;
  role: string;
  session: SessionResponse;
  settings: TenantSettings;
};

export const TenantAccessContext = createContext<TenantAccess | null>(null);

export function useOptionalTenantAccess(): TenantAccess | null {
  return useContext(TenantAccessContext);
}

export function useTenantAccess(): TenantAccess {
  const value = useOptionalTenantAccess();
  if (!value) {
    throw new Error('TenantAccessProvider is required');
  }
  return value;
}
