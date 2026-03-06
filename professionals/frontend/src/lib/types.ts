export type ProfessionalProfile = {
  id: string;
  party_id: string;
  bio: string;
  headline: string;
  public_slug: string;
  is_public: boolean;
  is_bookable: boolean;
  specialties: string[];
  created_at: string;
  updated_at: string;
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
  profile_id: string;
  status: 'scheduled' | 'in_progress' | 'completed' | 'cancelled';
  started_at: string;
  ended_at: string;
  summary: string;
  notes: SessionNote[];
  created_at: string;
  updated_at: string;
};

export type SessionNote = {
  id: string;
  session_id: string;
  content: string;
  author: string;
  created_at: string;
};
