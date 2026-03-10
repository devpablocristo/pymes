import { registerTokenProvider } from '@pymes/ts-pkg/http';
import type {
  Intake,
  OrgPreviewBootstrap,
  ProfessionalProfile,
  Session,
  SessionNote,
  ServiceLink,
  Specialty,
} from './professionalsTypes';
import { createVerticalRequest } from './verticalApi';

export function registerProfessionalsTokenProvider(provider: () => Promise<string | null>): void {
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
}): Intake {
  return {
    id: item.id,
    profile_id: item.profile_id,
    status: item.status,
    notes: typeof item.payload?.notes === 'string' ? item.payload.notes : '',
    created_at: item.created_at,
    updated_at: item.updated_at,
  };
}

const professionalRequest = createVerticalRequest({
  envVar: 'VITE_PROFESSIONALS_API_URL',
  fallbackPorts: [8181, 8081],
  translateError,
});

// ── Professionals ──

export async function getProfessionals(): Promise<{ items: ProfessionalProfile[] }> {
  return professionalRequest('/v1/professionals');
}

export async function createProfessional(data: {
  party_id: string;
  bio: string;
  headline: string;
  public_slug: string;
  is_public?: boolean;
  is_bookable?: boolean;
  accepts_new_clients?: boolean;
}): Promise<ProfessionalProfile> {
  return professionalRequest('/v1/professionals', { method: 'POST', body: data });
}

export async function getProfessional(id: string): Promise<ProfessionalProfile> {
  return professionalRequest(`/v1/professionals/${id}`);
}

export async function updateProfessional(
  id: string,
  data: Partial<{
    bio: string;
    headline: string;
    public_slug: string;
    is_public: boolean;
    is_bookable: boolean;
    accepts_new_clients: boolean;
  }>,
): Promise<ProfessionalProfile> {
  return professionalRequest(`/v1/professionals/${id}`, { method: 'PUT', body: data });
}

// ── Specialties ──

export async function getSpecialties(): Promise<{ items: Specialty[] }> {
  return professionalRequest('/v1/specialties');
}

export async function createSpecialty(data: {
  code: string;
  name: string;
  description: string;
  is_active?: boolean;
}): Promise<Specialty> {
  return professionalRequest('/v1/specialties', { method: 'POST', body: data });
}

export async function updateSpecialty(
  id: string,
  data: Partial<{ code: string; name: string; description: string; is_active: boolean }>,
): Promise<Specialty> {
  return professionalRequest(`/v1/specialties/${id}`, { method: 'PUT', body: data });
}

// ── Profile Services ──

export async function getProfileServices(id: string): Promise<{ items: ServiceLink[] }> {
  return professionalRequest(`/v1/professionals/${id}/services`);
}

export async function updateProfileServices(id: string, links: ServiceLink[]): Promise<void> {
  return professionalRequest(`/v1/professionals/${id}/services`, { method: 'PUT', body: { items: links } });
}

// ── Intakes ──

export async function getIntakes(): Promise<{ items: Intake[] }> {
  const response = await professionalRequest<{ items?: Array<{
    id: string;
    profile_id: string;
    status: 'draft' | 'submitted' | 'reviewed';
    payload?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
  }> }>('/v1/intakes');
  return { items: (response.items ?? []).map(mapIntake) };
}

export async function getIntake(id: string): Promise<Intake> {
  const response = await professionalRequest<{
    id: string;
    profile_id: string;
    status: 'draft' | 'submitted' | 'reviewed';
    payload?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
  }>(`/v1/intakes/${id}`);
  return mapIntake(response);
}

export async function createIntake(data: {
  profile_id: string;
  notes: string;
}): Promise<Intake> {
  const response = await professionalRequest<{
    id: string;
    profile_id: string;
    status: 'draft' | 'submitted' | 'reviewed';
    payload?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
  }>('/v1/intakes', {
    method: 'POST',
    body: { profile_id: data.profile_id, payload: { notes: data.notes } },
  });
  return mapIntake(response);
}

export async function updateIntake(
  id: string,
  data: Partial<{ notes: string }>,
): Promise<Intake> {
  const response = await professionalRequest<{
    id: string;
    profile_id: string;
    status: 'draft' | 'submitted' | 'reviewed';
    payload?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
  }>(`/v1/intakes/${id}`, {
    method: 'PUT',
    body: { payload: { notes: data.notes ?? '' } },
  });
  return mapIntake(response);
}

export async function submitIntake(id: string): Promise<Intake> {
  const response = await professionalRequest<{
    id: string;
    profile_id: string;
    status: 'draft' | 'submitted' | 'reviewed';
    payload?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
  }>(`/v1/intakes/${id}/submit`, { method: 'POST', body: {} });
  return mapIntake(response);
}

// ── Sessions ──

export async function getSessions(filters?: { status?: string; profile_id?: string }): Promise<{ items: Session[] }> {
  const params = new URLSearchParams();
  if (filters?.status) params.set('status', filters.status);
  if (filters?.profile_id) params.set('profile_id', filters.profile_id);
  const qs = params.toString();
  return professionalRequest(`/v1/sessions${qs ? `?${qs}` : ''}`);
}

export async function createSession(data: {
  appointment_id: string;
  profile_id: string;
  customer_party_id?: string;
  product_id?: string;
  started_at: string;
  summary?: string;
}): Promise<Session> {
  return professionalRequest('/v1/sessions', { method: 'POST', body: data });
}

export async function getSession(id: string): Promise<Session> {
  return professionalRequest(`/v1/sessions/${id}`);
}

export async function completeSession(id: string): Promise<Session> {
  return professionalRequest(`/v1/sessions/${id}/complete`, { method: 'POST', body: {} });
}

export async function addSessionNote(
  id: string,
  data: { body: string; title?: string; note_type?: string },
): Promise<SessionNote> {
  return professionalRequest(`/v1/sessions/${id}/notes`, { method: 'POST', body: data });
}

// ── Public ──

export async function getPublicProfessionals(orgSlug: string): Promise<{ items: ProfessionalProfile[] }> {
  return professionalRequest(`/v1/public/${orgSlug}/professionals`);
}

export async function getPublicPreviewBootstrap(): Promise<OrgPreviewBootstrap> {
  return professionalRequest('/v1/public-preview/bootstrap');
}
