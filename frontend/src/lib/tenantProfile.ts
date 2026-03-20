export type TeamSize = 'solo' | 'small' | 'medium' | 'large';
export type SellsType = 'products' | 'services' | 'both' | 'unsure';
export type PaymentMethod = 'cash' | 'transfer' | 'card' | 'mixed';
export type VerticalType = 'none' | 'professionals' | 'workshops';

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

export function getTenantProfile(): TenantProfile | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    return JSON.parse(raw) as TenantProfile;
  } catch {
    return null;
  }
}

export function saveTenantProfile(profile: TenantProfile): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(profile));
}

export function clearTenantProfile(): void {
  localStorage.removeItem(STORAGE_KEY);
}

export function hasCompletedOnboarding(): boolean {
  return getTenantProfile() !== null;
}
