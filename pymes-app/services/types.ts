export type SessionAuth = {
  org_id: string;
  tenant_id: string;
  role: string;
  product_role: string;
  scopes: string[];
  actor: string;
  auth_method: string;
  org_name: string;
  vertical: string;
  onboarding_completed_at: string | null;
};

export type SessionResponse = {
  auth: SessionAuth;
};

export type UserProfile = {
  id: string;
  name: string;
  given_name: string;
  family_name: string;
  email: string;
  phone: string | null;
};

export type UpdateProfileBody = Partial<
  Pick<UserProfile, 'name' | 'given_name' | 'family_name' | 'phone'>
>;
