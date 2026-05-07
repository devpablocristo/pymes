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
  tenant_id?: string;
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
    tenant_id: item.tenant_id,
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
  devPorts: [8181, 8081],
  translateError,
});

// ── Teachers ──

export async function getTeachers(filters?: { archived?: boolean }): Promise<{ items: TeacherProfile[] }> {
  return teachersRequest(`/v1/teachers/professionals${filters?.archived ? '/archived' : ''}`);
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
  metadata?: Record<string, unknown>;
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
    metadata: Record<string, unknown>;
  }>,
): Promise<TeacherProfile> {
  return teachersRequest(`/v1/teachers/professionals/${id}`, { method: 'PATCH', body: data });
}

export async function archiveTeacher(id: string): Promise<void> {
  await teachersRequest(`/v1/teachers/professionals/${id}/archive`, { method: 'POST', body: {} });
}

export async function restoreTeacher(id: string): Promise<void> {
  await teachersRequest(`/v1/teachers/professionals/${id}/restore`, { method: 'POST', body: {} });
}

export async function hardDeleteTeacher(id: string): Promise<void> {
  await teachersRequest(`/v1/teachers/professionals/${id}/hard`, { method: 'DELETE' });
}

// ── Specialties ──

export async function getTeacherSpecialties(filters?: { archived?: boolean }): Promise<{ items: TeacherSpecialty[] }> {
  return teachersRequest(`/v1/teachers/specialties${filters?.archived ? '/archived' : ''}`);
}

export async function createTeacherSpecialty(data: {
  code: string;
  name: string;
  description: string;
  is_active?: boolean;
  is_favorite?: boolean;
  tags?: string[];
  metadata?: Record<string, unknown>;
}): Promise<TeacherSpecialty> {
  return teachersRequest('/v1/teachers/specialties', { method: 'POST', body: data });
}

export async function updateTeacherSpecialty(
  id: string,
  data: Partial<{
    code: string;
    name: string;
    description: string;
    is_active: boolean;
    is_favorite: boolean;
    tags: string[];
    metadata: Record<string, unknown>;
  }>,
): Promise<TeacherSpecialty> {
  return teachersRequest(`/v1/teachers/specialties/${id}`, { method: 'PATCH', body: data });
}

export async function archiveTeacherSpecialty(id: string): Promise<void> {
  await teachersRequest(`/v1/teachers/specialties/${id}/archive`, { method: 'POST', body: {} });
}

export async function restoreTeacherSpecialty(id: string): Promise<void> {
  await teachersRequest(`/v1/teachers/specialties/${id}/restore`, { method: 'POST', body: {} });
}

export async function hardDeleteTeacherSpecialty(id: string): Promise<void> {
  await teachersRequest(`/v1/teachers/specialties/${id}/hard`, { method: 'DELETE' });
}

// ── Profile Services ──

export async function getTeacherServices(id: string): Promise<{ items: TeacherServiceLink[] }> {
  return teachersRequest(`/v1/teachers/professionals/${id}/services`);
}

export async function updateTeacherServices(id: string, links: TeacherServiceLink[]): Promise<void> {
  return teachersRequest(`/v1/teachers/professionals/${id}/services`, { method: 'PUT', body: { links } });
}

// ── Intakes ──

export async function getTeacherIntakes(filters?: { archived?: boolean }): Promise<{ items: TeacherIntake[] }> {
  const response = await teachersRequest<{
    items?: Array<{
      id: string;
      tenant_id?: string;
      booking_id?: string;
      profile_id: string;
      customer_party_id?: string;
      service_id?: string;
      status: 'draft' | 'submitted' | 'reviewed';
      payload?: Record<string, unknown>;
      created_at: string;
      updated_at: string;
    }>;
  }>(`/v1/teachers/intakes${filters?.archived ? '/archived' : ''}`);
  return { items: (response.items ?? []).map(mapIntake) };
}

export async function getTeacherIntake(id: string): Promise<TeacherIntake> {
  const response = await teachersRequest<{
    id: string;
    tenant_id?: string;
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
  payload: Record<string, unknown>;
  is_favorite?: boolean;
  tags?: string[];
}): Promise<TeacherIntake> {
  const response = await teachersRequest<{
    id: string;
    tenant_id?: string;
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
      payload: data.payload,
      is_favorite: data.is_favorite,
      tags: data.tags,
    },
  });
  return mapIntake(response);
}

export async function updateTeacherIntake(
  id: string,
  data: Partial<{ payload: Record<string, unknown>; is_favorite: boolean; tags: string[] }>,
): Promise<TeacherIntake> {
  const body: Record<string, unknown> = {};
  if (data.payload !== undefined) body.payload = data.payload;
  if (data.is_favorite !== undefined) body.is_favorite = data.is_favorite;
  if (data.tags !== undefined) body.tags = data.tags;
  const response = await teachersRequest<{
    id: string;
    tenant_id?: string;
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
    body,
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

export async function archiveTeacherIntake(id: string): Promise<void> {
  await teachersRequest(`/v1/teachers/intakes/${id}/archive`, { method: 'POST', body: {} });
}

export async function restoreTeacherIntake(id: string): Promise<void> {
  await teachersRequest(`/v1/teachers/intakes/${id}/restore`, { method: 'POST', body: {} });
}

export async function hardDeleteTeacherIntake(id: string): Promise<void> {
  await teachersRequest(`/v1/teachers/intakes/${id}/hard`, { method: 'DELETE' });
}

// ── Sessions ──

export async function getTeacherSessions(filters?: {
  status?: string;
  profile_id?: string;
  archived?: boolean;
}): Promise<{ items: TeacherSession[] }> {
  const params = new URLSearchParams();
  if (filters?.status) params.set('status', filters.status);
  if (filters?.profile_id) params.set('profile_id', filters.profile_id);
  const qs = params.toString();
  const path = `/v1/teachers/sessions${filters?.archived ? '/archived' : ''}`;
  return teachersRequest(`${path}${qs ? `?${qs}` : ''}`);
}

export async function createTeacherSession(data: {
  booking_id: string;
  profile_id: string;
  customer_party_id?: string;
  service_id?: string;
  started_at: string;
  summary?: string;
  metadata?: Record<string, unknown>;
}): Promise<TeacherSession> {
  return teachersRequest('/v1/teachers/sessions', { method: 'POST', body: data });
}

export async function getTeacherSession(id: string): Promise<TeacherSession> {
  return teachersRequest(`/v1/teachers/sessions/${id}`);
}

export async function updateTeacherSession(
  id: string,
  data: Partial<{
    booking_id: string;
    profile_id: string;
    customer_party_id: string | null;
    service_id: string | null;
    started_at: string;
    summary: string;
    metadata: Record<string, unknown>;
  }>,
): Promise<TeacherSession> {
  return teachersRequest(`/v1/teachers/sessions/${id}`, { method: 'PATCH', body: data });
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

export async function archiveTeacherSession(id: string): Promise<void> {
  await teachersRequest(`/v1/teachers/sessions/${id}/archive`, { method: 'POST', body: {} });
}

export async function restoreTeacherSession(id: string): Promise<void> {
  await teachersRequest(`/v1/teachers/sessions/${id}/restore`, { method: 'POST', body: {} });
}

export async function hardDeleteTeacherSession(id: string): Promise<void> {
  await teachersRequest(`/v1/teachers/sessions/${id}/hard`, { method: 'DELETE' });
}
