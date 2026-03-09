export type ProfessionalProfile = {
  id: string;
  party_id: string;
  bio: string;
  headline: string;
  public_slug: string;
  is_public: boolean;
  is_bookable: boolean;
  accepts_new_clients: boolean;
  specialties: Array<string | { id: string; code: string; name: string }>;
  created_at: string;
  updated_at: string;
};

export type OrgPreviewBootstrap = {
  org_id: string;
  slug: string;
  name: string;
  business_name: string;
};

export type Specialty = {
  id: string;
  code: string;
  name: string;
  description: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
};

export type ServiceLink = {
  specialty_id: string;
  specialty_code: string;
  specialty_name: string;
  enabled: boolean;
};

export type Intake = {
  id: string;
  profile_id: string;
  status: 'draft' | 'submitted' | 'reviewed';
  notes: string;
  created_at: string;
  updated_at: string;
};

export type Session = {
  id: string;
  appointment_id: string;
  profile_id: string;
  customer_party_id?: string;
  product_id?: string;
  status: 'scheduled' | 'active' | 'completed' | 'cancelled';
  started_at?: string;
  ended_at?: string;
  summary: string;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type SessionNote = {
  id: string;
  session_id: string;
  note_type: string;
  title: string;
  body: string;
  created_by: string;
  created_at: string;
};
