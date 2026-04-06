/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
import type { CrudPageConfig, CrudResourceConfigMap } from '../components/CrudPage';
import {
  addTeacherSessionNote,
  completeTeacherSession,
  createTeacher,
  createTeacherIntake,
  createTeacherSession,
  createTeacherSpecialty,
  getTeacherIntakes,
  getTeachers,
  getTeacherSessions,
  getTeacherSpecialties,
  submitTeacherIntake,
  updateTeacher,
  updateTeacherIntake,
  updateTeacherSpecialty,
} from '../lib/teachersApi';
import type { TeacherIntake, TeacherProfile, TeacherSession, TeacherSpecialty } from '../lib/teachersTypes';
import { buildConfiguredCrudPage, getCrudPageConfigFromMap, hasCrudResourceInMap } from './resourceConfigs.runtime';
import { asBoolean, asOptionalString, asString, formatDate, toRFC3339 } from './resourceConfigs.shared';

const resourceConfigs: CrudResourceConfigMap = {
  professionals: {
    label: 'teacher',
    labelPlural: 'teachers',
    labelPluralCap: 'Teachers',
    dataSource: {
      list: async () => (await getTeachers()).items ?? [],
      create: async (values) => {
        await createTeacher({
          party_id: asString(values.party_id),
          bio: asString(values.bio),
          headline: asString(values.headline),
          public_slug: asString(values.public_slug),
          is_public: asBoolean(values.is_public),
          is_bookable: asBoolean(values.is_bookable),
          accepts_new_clients: asBoolean(values.accepts_new_clients),
        });
      },
      update: async (row: TeacherProfile, values) => {
        await updateTeacher(row.id, {
          bio: asOptionalString(values.bio),
          headline: asOptionalString(values.headline),
          public_slug: asOptionalString(values.public_slug),
          is_public: asBoolean(values.is_public),
          is_bookable: asBoolean(values.is_bookable),
          accepts_new_clients: asBoolean(values.accepts_new_clients),
        });
      },
    },
    columns: [
      {
        key: 'headline',
        header: 'Teacher',
        className: 'cell-name',
        render: (_value, row: TeacherProfile) => (
          <>
            <strong>{row.headline || row.party_id}</strong>
            <div className="text-secondary">
              {row.public_slug || 'Sin slug'} · {row.party_id}
            </div>
          </>
        ),
      },
      {
        key: 'specialties',
        header: 'Especialidades',
        render: (value) =>
          ((value as TeacherProfile['specialties']) ?? [])
            .map((item) => (typeof item === 'string' ? item : item.name))
            .join(', ') || '---',
      },
      {
        key: 'is_public',
        header: 'Publico',
        render: (value) => (
          <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? 'Si' : 'No'}</span>
        ),
      },
      {
        key: 'is_bookable',
        header: 'Reservable',
        render: (value) => (
          <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? 'Si' : 'No'}</span>
        ),
      },
    ],
    formFields: [
      { key: 'party_id', label: 'Party ID', required: true, placeholder: 'UUID de la entidad' },
      { key: 'headline', label: 'Headline docente', placeholder: 'Teacher de ingles para secundaria' },
      { key: 'public_slug', label: 'Slug publico', placeholder: 'ana-perez' },
      { key: 'is_public', label: 'Visible al publico', type: 'checkbox' },
      { key: 'is_bookable', label: 'Reservable', type: 'checkbox' },
      { key: 'accepts_new_clients', label: 'Acepta nuevos alumnos', type: 'checkbox' },
      { key: 'bio', label: 'Bio', type: 'textarea', fullWidth: true },
    ],
    rowActions: [
      {
        id: 'toggle-public',
        label: 'Publicar',
        kind: 'secondary',
        onClick: async (row: TeacherProfile) => {
          await updateTeacher(row.id, { is_public: !row.is_public });
        },
      },
      {
        id: 'toggle-bookable',
        label: 'Reservable',
        kind: 'secondary',
        onClick: async (row: TeacherProfile) => {
          await updateTeacher(row.id, { is_bookable: !row.is_bookable });
        },
      },
    ],
    searchText: (row: TeacherProfile) =>
      [
        row.party_id,
        row.headline,
        row.public_slug,
        row.bio,
        row.specialties.map((item) => (typeof item === 'string' ? item : item.name)).join(', '),
      ]
        .filter(Boolean)
        .join(' '),
    toFormValues: (row: TeacherProfile) => ({
      party_id: row.party_id ?? '',
      headline: row.headline ?? '',
      public_slug: row.public_slug ?? '',
      bio: row.bio ?? '',
      is_public: row.is_public ?? false,
      is_bookable: row.is_bookable ?? false,
      accepts_new_clients: row.accepts_new_clients ?? true,
    }),
    isValid: (values) => asString(values.party_id).trim().length > 0,
  },
  specialties: {
    label: 'especialidad',
    labelPlural: 'especialidades',
    labelPluralCap: 'Especialidades',
    dataSource: {
      list: async () => (await getTeacherSpecialties()).items ?? [],
      create: async (values) => {
        await createTeacherSpecialty({
          code: asString(values.code),
          name: asString(values.name),
          description: asString(values.description),
          is_active: asBoolean(values.is_active),
        });
      },
      update: async (row: TeacherSpecialty, values) => {
        await updateTeacherSpecialty(row.id, {
          code: asOptionalString(values.code),
          name: asOptionalString(values.name),
          description: asOptionalString(values.description),
          is_active: asBoolean(values.is_active),
        });
      },
    },
    columns: [
      { key: 'code', header: 'Codigo' },
      { key: 'name', header: 'Nombre', className: 'cell-name' },
      { key: 'description', header: 'Descripcion' },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => (
          <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? 'Activa' : 'Inactiva'}</span>
        ),
      },
    ],
    formFields: [
      { key: 'code', label: 'Codigo', required: true, placeholder: 'PSY' },
      { key: 'name', label: 'Nombre', required: true, placeholder: 'Psicologia' },
      { key: 'description', label: 'Descripcion', type: 'textarea', fullWidth: true },
      { key: 'is_active', label: 'Activa', type: 'checkbox' },
    ],
    rowActions: [
      {
        id: 'toggle-active',
        label: 'Activar / pausar',
        kind: 'secondary',
        onClick: async (row: TeacherSpecialty) => {
          await updateTeacherSpecialty(row.id, { is_active: !row.is_active });
        },
      },
    ],
    searchText: (row: TeacherSpecialty) => [row.code, row.name, row.description].filter(Boolean).join(' '),
    toFormValues: (row: TeacherSpecialty) => ({
      code: row.code ?? '',
      name: row.name ?? '',
      description: row.description ?? '',
      is_active: row.is_active ?? true,
    }),
    isValid: (values) => asString(values.code).trim().length >= 2 && asString(values.name).trim().length >= 2,
  },
  intakes: {
    label: 'ingreso',
    labelPlural: 'ingresos',
    labelPluralCap: 'Ingresos',
    dataSource: {
      list: async () => (await getTeacherIntakes()).items ?? [],
      create: async (values) => {
        await createTeacherIntake({
          profile_id: asString(values.profile_id),
          notes: asString(values.notes),
        });
      },
      update: async (row: TeacherIntake, values) => {
        await updateTeacherIntake(row.id, { notes: asString(values.notes) });
      },
    },
    columns: [
      { key: 'profile_id', header: 'Teacher', className: 'cell-name' },
      {
        key: 'status',
        header: 'Estado',
        render: (value) => (
          <span
            className={`badge ${value === 'reviewed' ? 'badge-success' : value === 'submitted' ? 'badge-warning' : 'badge-neutral'}`}
          >
            {String(value)}
          </span>
        ),
      },
      { key: 'created_at', header: 'Creado', render: (value) => formatDate(String(value ?? '')) },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
    formFields: [
      { key: 'profile_id', label: 'Teacher ID', required: true, placeholder: 'UUID del teacher' },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    rowActions: [
      {
        id: 'submit',
        label: 'Enviar',
        kind: 'success',
        isVisible: (row: TeacherIntake) => row.status === 'draft',
        onClick: async (row: TeacherIntake) => {
          await submitTeacherIntake(row.id);
        },
      },
    ],
    searchText: (row: TeacherIntake) => [row.profile_id, row.status, row.notes].filter(Boolean).join(' '),
    toFormValues: (row: TeacherIntake) => ({
      profile_id: row.profile_id ?? '',
      notes: row.notes ?? '',
    }),
    isValid: (values) => asString(values.profile_id).trim().length > 0,
  },
  sessions: {
    label: 'sesion',
    labelPlural: 'sesiones',
    labelPluralCap: 'Sesiones',
    dataSource: {
      list: async () => (await getTeacherSessions()).items ?? [],
      create: async (values) => {
        await createTeacherSession({
          booking_id: asString(values.booking_id),
          profile_id: asString(values.profile_id),
          customer_party_id: asOptionalString(values.customer_party_id),
          service_id: asOptionalString(values.service_id),
          started_at: toRFC3339(values.started_at) ?? new Date().toISOString(),
          summary: asOptionalString(values.summary),
        });
      },
    },
    columns: [
      {
        key: 'profile_id',
        header: 'Sesion',
        className: 'cell-name',
        render: (_value, row: TeacherSession) => (
          <>
            <strong>{row.profile_id}</strong>
            <div className="text-secondary">
              {row.booking_id} · {row.summary || 'Sin resumen'}
            </div>
          </>
        ),
      },
      {
        key: 'status',
        header: 'Estado',
        render: (value) => (
          <span
            className={`badge ${value === 'completed' ? 'badge-success' : value === 'active' ? 'badge-warning' : 'badge-neutral'}`}
          >
            {String(value)}
          </span>
        ),
      },
      { key: 'started_at', header: 'Inicio', render: (value) => formatDate(String(value ?? '')) },
      { key: 'ended_at', header: 'Fin', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'booking_id', label: 'Booking ID', required: true, placeholder: 'UUID del turno' },
      { key: 'profile_id', label: 'Teacher ID', required: true, placeholder: 'UUID del teacher' },
      { key: 'customer_party_id', label: 'Customer party ID' },
      { key: 'service_id', label: 'Service ID' },
      { key: 'started_at', label: 'Inicio', type: 'datetime-local', required: true },
      { key: 'summary', label: 'Resumen', type: 'textarea', fullWidth: true },
    ],
    rowActions: [
      {
        id: 'complete',
        label: 'Completar',
        kind: 'success',
        isVisible: (row: TeacherSession) => row.status === 'scheduled' || row.status === 'active',
        onClick: async (row: TeacherSession) => {
          await completeTeacherSession(row.id);
        },
      },
      {
        id: 'note',
        label: 'Nota',
        kind: 'secondary',
        onClick: async (row: TeacherSession) => {
          const body = window.prompt('Nota de la sesion');
          if (!body || !body.trim()) return;
          const title = window.prompt('Titulo de la nota (opcional)') ?? '';
          await addTeacherSessionNote(row.id, { body: body.trim(), title: title.trim() || undefined });
        },
      },
    ],
    searchText: (row: TeacherSession) =>
      [row.booking_id, row.profile_id, row.status, row.summary].filter(Boolean).join(' '),
    toFormValues: () => ({
      booking_id: '',
      profile_id: '',
      customer_party_id: '',
      service_id: '',
      started_at: '',
      summary: '',
    }),
    isValid: (values) =>
      asString(values.booking_id).trim().length > 0 &&
      asString(values.profile_id).trim().length > 0 &&
      Boolean(toRFC3339(values.started_at)),
  },
};

resourceConfigs.teachers = resourceConfigs.professionals;

export const ConfiguredCrudPage = buildConfiguredCrudPage(resourceConfigs);

export function hasCrudResource(resourceId: string): boolean {
  return hasCrudResourceInMap(resourceConfigs, resourceId);
}

export function getCrudPageConfig<TRecord extends { id: string } = { id: string }>(
  resourceId: string,
): CrudPageConfig<TRecord> | null {
  return getCrudPageConfigFromMap<TRecord>(resourceConfigs, resourceId);
}
