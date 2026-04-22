import { registerTokenProvider } from '@devpablocristo/core-authn/http/fetch';
import type {
  TeacherIntake,
  TeacherProfile,
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
  org_id?: string;
  booking_id?: string;
  profile_id: string;
  customer_party_id?: string;
  service_id?: string;
  status: 'draft' | 'submitted' | 'reviewed';
  is_favorite?: boolean;
  tags?: string[];
  payload?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}): TeacherIntake {
  return {
    id: item.id,
    org_id: item.org_id,
    booking_id: item.booking_id,
    profile_id: item.profile_id,
    customer_party_id: item.customer_party_id,
    service_id: item.service_id,
    status: item.status,
    notes: typeof item.payload?.notes === 'string' ? item.payload.notes : '',
    is_favorite: item.is_favorite,
    tags: item.tags,
    payload: item.payload,
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
  is_favorite?: boolean;
  tags?: string[];
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
    is_favorite: boolean;
    tags: string[];
  }>,
): Promise<TeacherProfile> {
  return teachersRequest(`/v1/teachers/professionals/${id}`, { method: 'PATCH', body: data });
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
  is_favorite?: boolean;
  tags?: string[];
}): Promise<TeacherSpecialty> {
  return teachersRequest('/v1/teachers/specialties', { method: 'POST', body: data });
}

export async function updateTeacherSpecialty(
  id: string,
  data: Partial<{ code: string; name: string; description: string; is_active: boolean; is_favorite: boolean; tags: string[] }>,
): Promise<TeacherSpecialty> {
  return teachersRequest(`/v1/teachers/specialties/${id}`, { method: 'PATCH', body: data });
}

// ── Profile Services ──

export async function getTeacherServices(id: string): Promise<{ items: TeacherServiceLink[] }> {
  return teachersRequest(`/v1/teachers/professionals/${id}/services`);
}

export async function updateTeacherServices(id: string, links: TeacherServiceLink[]): Promise<void> {
  return teachersRequest(`/v1/teachers/professionals/${id}/services`, { method: 'PUT', body: { links } });
}

// ── Intakes ──

export async function getTeacherIntakes(): Promise<{ items: TeacherIntake[] }> {
  const response = await teachersRequest<{
    items?: Array<{
      id: string;
      org_id?: string;
      booking_id?: string;
      profile_id: string;
      customer_party_id?: string;
      service_id?: string;
      status: 'draft' | 'submitted' | 'reviewed';
      payload?: Record<string, unknown>;
      created_at: string;
      updated_at: string;
    }>;
  }>('/v1/teachers/intakes');
  return { items: (response.items ?? []).map(mapIntake) };
}

export async function getTeacherIntake(id: string): Promise<TeacherIntake> {
  const response = await teachersRequest<{
    id: string;
    org_id?: string;
    booking_id?: string;
    profile_id: string;
    customer_party_id?: string;
    service_id?: string;
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
  is_favorite?: boolean;
  tags?: string[];
}): Promise<TeacherIntake> {
  const response = await teachersRequest<{
    id: string;
    org_id?: string;
    booking_id?: string;
    profile_id: string;
    customer_party_id?: string;
    service_id?: string;
    status: 'draft' | 'submitted' | 'reviewed';
    is_favorite?: boolean;
    tags?: string[];
    payload?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
  }>('/v1/teachers/intakes', {
    method: 'POST',
    body: {
      profile_id: data.profile_id,
      payload: { notes: data.notes },
      is_favorite: data.is_favorite,
      tags: data.tags,
    },
  });
  return mapIntake(response);
}

export async function updateTeacherIntake(
  id: string,
  data: Partial<{ notes: string; is_favorite: boolean; tags: string[] }>,
): Promise<TeacherIntake> {
  const response = await teachersRequest<{
    id: string;
    org_id?: string;
    booking_id?: string;
    profile_id: string;
    customer_party_id?: string;
    service_id?: string;
    status: 'draft' | 'submitted' | 'reviewed';
    is_favorite?: boolean;
    tags?: string[];
    payload?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
  }>(`/v1/teachers/intakes/${id}`, {
    method: 'PATCH',
    body: {
      payload: { notes: data.notes ?? '' },
      is_favorite: data.is_favorite,
      tags: data.tags,
    },
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

export async function getTeacherSessions(filters?: {
  status?: string;
  profile_id?: string;
}): Promise<{ items: TeacherSession[] }> {
  const params = new URLSearchParams();
  if (filters?.status) params.set('status', filters.status);
  if (filters?.profile_id) params.set('profile_id', filters.profile_id);
  const qs = params.toString();
  return teachersRequest(`/v1/teachers/sessions${qs ? `?${qs}` : ''}`);
}

export async function createTeacherSession(data: {
  booking_id: string;
  profile_id: string;
  customer_party_id?: string;
  service_id?: string;
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
