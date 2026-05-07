import { createVerticalRequest } from './verticalApi';
import type { OccupationalExamStatus, OccupationalExamType, OccupationalHealthExam } from './medicalTypes';

function translateMedicalError(message: string): string {
  const trimmed = message.trim();
  switch (trimmed) {
    case '404 page not found':
      return 'La ruta no existe en el backend de medical.';
    case 'organization not found':
      return 'No se encontró la organización.';
    case 'invalid org':
    case 'invalid org identifier':
      return 'No hay una empresa válida en la sesión para Medicina laboral. Con Clerk: completá el onboarding, recargá la página o cerrá sesión y volvé a entrar para renovar el token.';
    default:
      return trimmed;
  }
}

export const medicalRequest = createVerticalRequest({
  envVar: 'VITE_MEDICAL_API_URL',
  devPorts: [8585, 8085],
  translateError: translateMedicalError,
  timeoutMs: 60_000,
  timeoutMessage:
    'El backend de medical no respondió a tiempo. Levantá medical-backend (puerto 8585), revisá VITE_MEDICAL_API_URL y que las migraciones estén aplicadas.',
});

export async function listOccupationalHealthExams(filters: {
  search?: string;
  status?: string;
  archived?: boolean;
} = {}): Promise<{ items: OccupationalHealthExam[]; total: number }> {
  const params = new URLSearchParams();
  if (filters.search?.trim()) params.set('search', filters.search.trim());
  if (filters.status?.trim()) params.set('status', filters.status.trim());
  if (filters.archived) params.set('archived', 'true');
  const query = params.toString();
  return medicalRequest(`/v1/medical/occupational-health/exams${query ? `?${query}` : ''}`);
}

export async function createOccupationalHealthExam(data: {
  patient_name: string;
  patient_document?: string;
  employer_name?: string;
  client_name?: string;
  payment_method?: string;
  exam_type?: OccupationalExamType;
  status?: OccupationalExamStatus;
  scheduled_at?: string | null;
  result?: string;
  notes?: string;
  is_favorite?: boolean;
  tags?: string[];
  image_urls?: string[];
}): Promise<OccupationalHealthExam> {
  return medicalRequest('/v1/medical/occupational-health/exams', { method: 'POST', body: data });
}

export async function updateOccupationalHealthExam(
  id: string,
  data: Partial<{
    patient_name: string;
    patient_document: string;
    employer_name: string;
    client_name: string;
    payment_method: string;
    exam_type: OccupationalExamType;
    status: OccupationalExamStatus;
    scheduled_at: string | null;
    completed_at: string | null;
    result: string;
    notes: string;
    is_favorite: boolean;
    tags: string[];
    image_urls: string[];
  }>,
): Promise<OccupationalHealthExam> {
  return medicalRequest(`/v1/medical/occupational-health/exams/${id}`, { method: 'PATCH', body: data });
}

export async function archiveOccupationalHealthExam(id: string): Promise<void> {
  await medicalRequest(`/v1/medical/occupational-health/exams/${id}`, { method: 'DELETE' });
}

export async function restoreOccupationalHealthExam(id: string): Promise<void> {
  await medicalRequest(`/v1/medical/occupational-health/exams/${id}/restore`, { method: 'POST', body: {} });
}

export async function hardDeleteOccupationalHealthExam(id: string): Promise<void> {
  await medicalRequest(`/v1/medical/occupational-health/exams/${id}/hard`, { method: 'DELETE' });
}
