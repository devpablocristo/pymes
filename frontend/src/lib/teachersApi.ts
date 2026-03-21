import { registerTokenProvider } from '@devpablocristo/core-authn/http/fetch';
import type {
  TeacherIntake,
  TeacherProfile,
  TeachersPreviewBootstrap,
  TeacherServiceLink,
  TeacherSession,
  TeacherSessionNote,
  TeacherSpecialty,
} from './teachersTypes';
import { createVerticalRequest } from './verticalApi';

export function registerTeachersTokenProvider(provider: () => Promise<string | null>): void {
  registerTokenProvider(provider);
}

function translateError(message: string): string {
  const trimmed = message.trim();
  switch (trimmed) {
    case '404 page not found':
      return 'La ruta no existe en el backend de profesionales.';
    case 'organization not found':
      return 'No se encontro la organizacion.';
    default:
      return trimmed;
  }
}

function mapIntake(item: {
  id: string;
  profile_id: string;
  status: 'draft' | 'submitted' | 'reviewed';
  payload?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}): TeacherIntake {
  return {
    id: item.id,
    profile_id: item.profile_id,
    status: item.status,
    notes: typeof item.payload?.notes === 'string' ? item.payload.notes : '',
    created_at: item.created_at,
    updated_at: item.updated_at,
  };
}

const teachersRequest = createVerticalRequest({
  envVar: 'VITE_PROFESSIONALS_API_URL',
  fallbackPorts: [8181, 8081],
  translateError,
});

// ── Teachers ──

export async function getTeachers(): Promise<{ items: TeacherProfile[] }> {
  return teachersRequest('/v1/teachers/professionals');
}

export async function createTeacher(data: {
  party_id: string;
  bio: string;
  headline: string;
  public_slug: string;
  is_public?: boolean;
  is_bookable?: boolean;
  accepts_new_clients?: boolean;
}): Promise<TeacherProfile> {
  return teachersRequest('/v1/teachers/professionals', { method: 'POST', body: data });
}

export async function getTeacher(id: string): Promise<TeacherProfile> {
  return teachersRequest(`/v1/teachers/professionals/${id}`);
}

export async function updateTeacher(
  id: string,
  data: Partial<{
    bio: string;
    headline: string;
    public_slug: string;
    is_public: boolean;
    is_bookable: boolean;
    accepts_new_clients: boolean;
  }>,
): Promise<TeacherProfile> {
  return teachersRequest(`/v1/teachers/professionals/${id}`, { method: 'PUT', body: data });
}

// ── Specialties ──

export async function getTeacherSpecialties(): Promise<{ items: TeacherSpecialty[] }> {
  return teachersRequest('/v1/teachers/specialties');
}

export async function createTeacherSpecialty(data: {
  code: string;
  name: string;
  description: string;
  is_active?: boolean;
}): Promise<TeacherSpecialty> {
  return teachersRequest('/v1/teachers/specialties', { method: 'POST', body: data });
}

export async function updateTeacherSpecialty(
  id: string,
  data: Partial<{ code: string; name: string; description: string; is_active: boolean }>,
): Promise<TeacherSpecialty> {
  return teachersRequest(`/v1/teachers/specialties/${id}`, { method: 'PUT', body: data });
}

// ── Profile Services ──

export async function getTeacherServices(id: string): Promise<{ items: TeacherServiceLink[] }> {
  return teachersRequest(`/v1/teachers/professionals/${id}/services`);
}

export async function updateTeacherServices(id: string, links: TeacherServiceLink[]): Promise<void> {
  return teachersRequest(`/v1/teachers/professionals/${id}/services`, { method: 'PUT', body: { items: links } });
}

// ── Intakes ──

export async function getTeacherIntakes(): Promise<{ items: TeacherIntake[] }> {
  const response = await teachersRequest<{ items?: Array<{
    id: string;
    profile_id: string;
    status: 'draft' | 'submitted' | 'reviewed';
    payload?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
  }> }>('/v1/teachers/intakes');
  return { items: (response.items ?? []).map(mapIntake) };
}

export async function getTeacherIntake(id: string): Promise<TeacherIntake> {
  const response = await teachersRequest<{
    id: string;
    profile_id: string;
    status: 'draft' | 'submitted' | 'reviewed';
    payload?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
  }>(`/v1/teachers/intakes/${id}`);
  return mapIntake(response);
}

export async function createTeacherIntake(data: {
  profile_id: string;
  notes: string;
}): Promise<TeacherIntake> {
  const response = await teachersRequest<{
    id: string;
    profile_id: string;
    status: 'draft' | 'submitted' | 'reviewed';
    payload?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
  }>('/v1/teachers/intakes', {
    method: 'POST',
    body: { profile_id: data.profile_id, payload: { notes: data.notes } },
  });
  return mapIntake(response);
}

export async function updateTeacherIntake(
  id: string,
  data: Partial<{ notes: string }>,
): Promise<TeacherIntake> {
  const response = await teachersRequest<{
    id: string;
    profile_id: string;
    status: 'draft' | 'submitted' | 'reviewed';
    payload?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
  }>(`/v1/teachers/intakes/${id}`, {
    method: 'PUT',
    body: { payload: { notes: data.notes ?? '' } },
  });
  return mapIntake(response);
}

export async function submitTeacherIntake(id: string): Promise<TeacherIntake> {
  const response = await teachersRequest<{
    id: string;
    profile_id: string;
    status: 'draft' | 'submitted' | 'reviewed';
    payload?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
  }>(`/v1/teachers/intakes/${id}/submit`, { method: 'POST', body: {} });
  return mapIntake(response);
}

// ── Sessions ──

export async function getTeacherSessions(filters?: { status?: string; profile_id?: string }): Promise<{ items: TeacherSession[] }> {
  const params = new URLSearchParams();
  if (filters?.status) params.set('status', filters.status);
  if (filters?.profile_id) params.set('profile_id', filters.profile_id);
  const qs = params.toString();
  return teachersRequest(`/v1/teachers/sessions${qs ? `?${qs}` : ''}`);
}

export async function createTeacherSession(data: {
  appointment_id: string;
  profile_id: string;
  customer_party_id?: string;
  product_id?: string;
  started_at: string;
  summary?: string;
}): Promise<TeacherSession> {
  return teachersRequest('/v1/teachers/sessions', { method: 'POST', body: data });
}

export async function getTeacherSession(id: string): Promise<TeacherSession> {
  return teachersRequest(`/v1/teachers/sessions/${id}`);
}

export async function completeTeacherSession(id: string): Promise<TeacherSession> {
  return teachersRequest(`/v1/teachers/sessions/${id}/complete`, { method: 'POST', body: {} });
}

export async function addTeacherSessionNote(
  id: string,
  data: { body: string; title?: string; note_type?: string },
): Promise<TeacherSessionNote> {
  return teachersRequest(`/v1/teachers/sessions/${id}/notes`, { method: 'POST', body: data });
}

// ── Public ──

export async function getPublicTeachers(orgSlug: string): Promise<{ items: TeacherProfile[] }> {
  return teachersRequest(`/v1/public/${orgSlug}/teachers`);
}

export async function getTeachersPreviewBootstrap(): Promise<TeachersPreviewBootstrap> {
  return teachersRequest('/v1/teachers/public-preview/bootstrap');
}

export const registerProfessionalsTokenProvider = registerTeachersTokenProvider;
export const getProfessionals = getTeachers;
export const createProfessional = createTeacher;
export const getProfessional = getTeacher;
export const updateProfessional = updateTeacher;
export const getSpecialties = getTeacherSpecialties;
export const createSpecialty = createTeacherSpecialty;
export const updateSpecialty = updateTeacherSpecialty;
export const getProfileServices = getTeacherServices;
export const updateProfileServices = updateTeacherServices;
export const getIntakes = getTeacherIntakes;
export const getIntake = getTeacherIntake;
export const createIntake = createTeacherIntake;
export const updateIntake = updateTeacherIntake;
export const submitIntake = submitTeacherIntake;
export const getSessions = getTeacherSessions;
export const createSession = createTeacherSession;
export const getSession = getTeacherSession;
export const completeSession = completeTeacherSession;
export const addSessionNote = addTeacherSessionNote;
export const getPublicProfessionals = getPublicTeachers;
export const getPublicPreviewBootstrap = getTeachersPreviewBootstrap;
