export type OccupationalExamStatus = 'pending' | 'scheduled' | 'completed' | 'cancelled';

export type OccupationalExamType = 'pre_employment' | 'periodic' | 'return_to_work' | 'exit' | 'other';

export type OccupationalHealthExam = {
  id: string;
  org_id: string;
  patient_name: string;
  patient_document: string;
  employer_name: string;
  client_name: string;
  payment_method: string;
  exam_type: OccupationalExamType;
  status: OccupationalExamStatus;
  scheduled_at?: string | null;
  completed_at?: string | null;
  result: string;
  notes: string;
  is_favorite?: boolean;
  tags?: string[];
  image_urls?: string[];
  created_at: string;
  updated_at: string;
};
