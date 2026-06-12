import type { ReactNode } from 'react';
import { TenantAccessContext, type TenantAccess } from './tenantAccessContext';

export function TenantAccessProvider({ value, children }: { value: TenantAccess; children: ReactNode }) {
  return <TenantAccessContext.Provider value={value}>{children}</TenantAccessContext.Provider>;
}
