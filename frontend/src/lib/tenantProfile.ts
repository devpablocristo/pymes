import { createBrowserStorageNamespace } from '@devpablocristo/core-browser/storage';

export type TeamSize = 'solo' | 'small' | 'medium' | 'large';
export type SellsType = 'products' | 'services' | 'both' | 'unsure';
export type PaymentMethod = 'cash' | 'transfer' | 'card' | 'mixed';
export type VerticalType = 'none' | 'professionals' | 'workshops' | 'beauty' | 'restaurants';

export type TenantProfile = {
  businessName: string;
  teamSize: TeamSize;
  sells: SellsType;
  clientLabel: string;
  usesScheduling: boolean;
  usesBilling: boolean;
  currency: string;
  paymentMethod: PaymentMethod;
  vertical: VerticalType;
  completedAt: string;
};

const STORAGE_KEY = 'pymes:tenant_profile';
const storage = createBrowserStorageNamespace({ namespace: 'pymes-ui', hostAware: false });

export function getTenantProfile(): TenantProfile | null {
  return storage.getJSON<TenantProfile>(STORAGE_KEY);
}

export function saveTenantProfile(profile: TenantProfile): void {
  storage.setJSON(STORAGE_KEY, profile);
}

export function clearTenantProfile(): void {
  storage.remove(STORAGE_KEY);
}

export function hasCompletedOnboarding(): boolean {
  return getTenantProfile() !== null;
}
