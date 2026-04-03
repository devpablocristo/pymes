import { createBrowserStorageNamespace } from '@devpablocristo/core-browser/storage';
import type { TenantSettings } from './types';

export type TeamSize = 'solo' | 'small' | 'medium' | 'large';
export type SellsType = 'products' | 'services' | 'both' | 'unsure';
export type PaymentMethod = 'cash' | 'transfer' | 'card' | 'mixed';
export type VerticalType = 'none' | 'professionals' | 'workshops' | 'bike_shop' | 'beauty' | 'restaurants';

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

export function tenantProfileFromSettings(settings: TenantSettings): TenantProfile | null {
  const completedAt = settings.onboarding_completed_at?.trim();
  if (!completedAt) {
    return null;
  }

  const teamSize = settings.team_size?.trim();
  const sells = settings.sells?.trim();
  const paymentMethod = settings.payment_method?.trim();
  const vertical = settings.vertical?.trim();
  if (!teamSize || !sells || !paymentMethod || !vertical) {
    return null;
  }

  return {
    businessName: settings.business_name?.trim() || '',
    teamSize: teamSize as TeamSize,
    sells: sells as SellsType,
    clientLabel: settings.client_label?.trim() || 'clientes',
    usesScheduling: Boolean(settings.scheduling_enabled),
    usesBilling: Boolean(settings.uses_billing),
    currency: settings.currency?.trim() || 'ARS',
    paymentMethod: paymentMethod as PaymentMethod,
    vertical: vertical as VerticalType,
    completedAt,
  };
}

export function syncTenantProfileFromSettings(settings: TenantSettings): TenantProfile | null {
  const profile = tenantProfileFromSettings(settings);
  if (!profile) {
    clearTenantProfile();
    return null;
  }
  saveTenantProfile(profile);
  return profile;
}
