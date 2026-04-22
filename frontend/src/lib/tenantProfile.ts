import { createBrowserStorageNamespace } from '@devpablocristo/core-browser/storage';
import type { TenantSettings } from './types';

export type TeamSize = 'solo' | 'small' | 'medium' | 'large';
export type SellsType = 'products' | 'services' | 'both' | 'unsure';
export type PaymentMethod = 'cash' | 'transfer' | 'card' | 'mixed';
export type VerticalType = 'none' | 'professionals' | 'workshops' | 'beauty' | 'restaurants';
export type SubVerticalType =
  | 'teachers'
  | 'consulting'
  | 'auto_repair'
  | 'bike_shop'
  | 'salon'
  | 'barbershop'
  | 'aesthetics'
  | 'restaurant'
  | 'bar'
  | 'cafe';

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
  subVertical?: SubVerticalType;
  completedAt: string;
};

const SUB_VERTICAL_BY_VERTICAL: Partial<Record<VerticalType, readonly SubVerticalType[]>> = {
  professionals: ['teachers', 'consulting'],
  workshops: ['auto_repair', 'bike_shop'],
  beauty: ['salon', 'barbershop', 'aesthetics'],
  restaurants: ['restaurant', 'bar', 'cafe'],
};

const STORAGE_KEY = 'pymes:tenant_profile';
const storage = createBrowserStorageNamespace({ namespace: 'pymes-ui', hostAware: false });

function normalizeTenantProfile(profile: TenantProfile | null): TenantProfile | null {
  if (!profile) {
    return null;
  }

  const { subVertical: rawSubVertical, ...rest } = profile;
  const rawVertical = profile.vertical as VerticalType | 'bike_shop';
  const normalizedVertical: VerticalType = rawVertical === 'bike_shop' ? 'workshops' : rawVertical;
  const normalizedSubVertical =
    rawVertical === 'bike_shop'
      ? 'bike_shop'
      : rawSubVertical && SUB_VERTICAL_BY_VERTICAL[normalizedVertical]?.includes(rawSubVertical)
        ? rawSubVertical
        : undefined;

  return {
    ...rest,
    vertical: normalizedVertical,
    ...(normalizedSubVertical ? { subVertical: normalizedSubVertical } : {}),
  };
}

export function getTenantProfile(): TenantProfile | null {
  return normalizeTenantProfile(storage.getJSON<TenantProfile>(STORAGE_KEY));
}

export function saveTenantProfile(profile: TenantProfile): void {
  const normalized = normalizeTenantProfile(profile);
  if (!normalized) {
    storage.remove(STORAGE_KEY);
    return;
  }
  storage.setJSON(STORAGE_KEY, normalized);
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

  const previousProfile = getTenantProfile();
  const normalizedVertical = (vertical === 'bike_shop' ? 'workshops' : vertical) as VerticalType;
  // Si el backend devuelve `vertical='bike_shop'` (alias directo del sub-vertical),
  // lo promovemos a subVertical aunque no haya profile previo. Sin este fallback,
  // el primer sync pierde la marca y el routing de work-orders cae en autoreparación.
  const inferredSubVertical: SubVerticalType | undefined = vertical === 'bike_shop' ? 'bike_shop' : undefined;
  const preservedSubVertical =
    inferredSubVertical ??
    (previousProfile?.vertical === normalizedVertical &&
    previousProfile.subVertical &&
    SUB_VERTICAL_BY_VERTICAL[normalizedVertical]?.includes(previousProfile.subVertical)
      ? previousProfile.subVertical
      : undefined);

  return {
    businessName: settings.business_name?.trim() || '',
    teamSize: teamSize as TeamSize,
    sells: sells as SellsType,
    clientLabel: settings.client_label?.trim() || 'clientes',
    usesScheduling: Boolean(settings.scheduling_enabled),
    usesBilling: Boolean(settings.uses_billing),
    currency: settings.currency?.trim() || 'ARS',
    paymentMethod: paymentMethod as PaymentMethod,
    vertical: normalizedVertical,
    ...(preservedSubVertical ? { subVertical: preservedSubVertical } : {}),
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
