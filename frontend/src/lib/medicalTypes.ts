export type OccupationalExamStatus = 'pending' | 'scheduled' | 'completed' | 'cancelled';

export type OccupationalExamType = 'pre_employment' | 'periodic' | 'return_to_work' | 'exit' | 'other';

export type OccupationalHealthExam = {
  id: string;
  tenant_id: string;
  patient_name: string;
  patient_document: string;
  employer_name: string;
  exam_type: OccupationalExamType;
  status: OccupationalExamStatus;
  scheduled_at?: string | null;
  completed_at?: string | null;
  result: string;
  notes: string;
  created_at: string;
  updated_at: string;
};

