import type { CrudFieldValue, CrudFormValues, CrudPageConfig } from '../../components/CrudPage';
import {
  archiveOccupationalHealthExam,
  createOccupationalHealthExam,
  hardDeleteOccupationalHealthExam,
  listOccupationalHealthExams,
  restoreOccupationalHealthExam,
  updateOccupationalHealthExam,
} from '../../lib/medicalApi';
import type { OccupationalExamStatus, OccupationalExamType, OccupationalHealthExam } from '../../lib/medicalTypes';
import { PymesSimpleCrudListModeContent } from '../../crud/PymesSimpleCrudListModeContent';
import { asOptionalString, asString, formatDate, toDateTimeInput, toRFC3339 } from '../../crud/resourceConfigs.shared';
import { buildFullyConnectedStatusStateMachine, buildStandardCrudViewModes } from '../crud';

export const occupationalExamTypeLabels: Record<OccupationalExamType, string> = {
  pre_employment: 'Preocupacional',
  periodic: 'Periódico',
  return_to_work: 'Reintegro',
  exit: 'Egreso',
  other: 'Otro',
};

export const occupationalExamStatusLabels: Record<OccupationalExamStatus, string> = {
  pending: 'Pendiente',
  scheduled: 'Agendado',
  completed: 'Completo',
  cancelled: 'Cancelado',
};

function examStatusBadge(status: OccupationalExamStatus) {
  const badgeClass =
    status === 'completed'
      ? 'badge-success'
      : status === 'scheduled'
        ? 'badge-warning'
        : status === 'cancelled'
          ? 'badge-danger'
          : 'badge-neutral';
  return <span className={`badge ${badgeClass}`}>{occupationalExamStatusLabels[status] ?? status}</span>;
}

function toExamBody(values: Record<string, CrudFieldValue | undefined>) {
  return {
    patient_name: asString(values.patient_name).trim(),
    patient_document: asOptionalString(values.patient_document) ?? '',
    employer_name: asOptionalString(values.employer_name) ?? '',
    exam_type: (asOptionalString(values.exam_type) ?? 'pre_employment') as OccupationalExamType,
    status: (asOptionalString(values.status) ?? 'pending') as OccupationalExamStatus,
    scheduled_at: toRFC3339(values.scheduled_at) ?? null,
    result: asOptionalString(values.result) ?? '',
    notes: asOptionalString(values.notes) ?? '',
  };
}

export function createOccupationalHealthExamsCrudConfig(): CrudPageConfig<OccupationalHealthExam> {
  const stateMachine = buildFullyConnectedStatusStateMachine<OccupationalHealthExam>([
    { value: 'pending', label: 'Pendiente', badgeVariant: 'default' },
    { value: 'scheduled', label: 'Agendado', badgeVariant: 'warning' },
    { value: 'completed', label: 'Completo', badgeVariant: 'success' },
    { value: 'cancelled', label: 'Cancelado', badgeVariant: 'danger' },
  ]);

  return {
    viewModes: buildStandardCrudViewModes(() => <PymesSimpleCrudListModeContent resourceId="occupationalHealthExams" />, {
      ariaLabel: 'Vista exámenes laborales',
    }),
    label: 'examen laboral',
    labelPlural: 'exámenes laborales',
    labelPluralCap: 'Medicina laboral',
    createLabel: '+ Nuevo examen',
    searchPlaceholder: 'Buscar trabajador, DNI o empresa...',
    emptyState: 'No hay exámenes laborales para mostrar.',
    supportsArchived: true,
    allowCreate: true,
    allowEdit: true,
    allowDelete: true,
    allowRestore: true,
    allowHardDelete: true,
    dataSource: {
      list: async ({ archived }) => {
        const data = await listOccupationalHealthExams({ archived: Boolean(archived) });
        return data.items ?? [];
      },
      create: async (values) => {
        await createOccupationalHealthExam(toExamBody(values));
      },
      update: async (row, values) => {
        await updateOccupationalHealthExam(row.id, toExamBody(values));
      },
      deleteItem: async (row) => {
        await archiveOccupationalHealthExam(row.id);
      },
      restore: async (row) => {
        await restoreOccupationalHealthExam(row.id);
      },
      hardDelete: async (row) => {
        await hardDeleteOccupationalHealthExam(row.id);
      },
    },
    stateMachine,
    kanban: {
      card: {
        title: (row) => row.patient_name || row.id,
        subtitle: (row) => row.employer_name || row.patient_document || 'Sin empresa',
        meta: (row) => occupationalExamTypeLabels[row.exam_type] ?? row.exam_type,
      },
      createFooterLabel: 'Añadir examen',
      persistMove: async ({ row, nextValue }) => {
        const status = nextValue as OccupationalExamStatus;
        const completedAt = status === 'completed' ? new Date().toISOString() : null;
        const updated = await updateOccupationalHealthExam(row.id, { status, completed_at: completedAt });
        return updated;
      },
    },
    columns: [
      { key: 'patient_name', header: 'Trabajador', className: 'cell-name' },
      { key: 'patient_document', header: 'Documento', render: (_value, row) => row.patient_document.trim() || '—' },
      { key: 'employer_name', header: 'Empresa', render: (_value, row) => row.employer_name.trim() || '—' },
      { key: 'exam_type', header: 'Tipo', render: (_value, row) => occupationalExamTypeLabels[row.exam_type] ?? row.exam_type },
      { key: 'status', header: 'Estado', render: (_value, row) => examStatusBadge(row.status) },
      { key: 'scheduled_at', header: 'Fecha', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'patient_name', label: 'Trabajador', required: true, placeholder: 'Nombre completo' },
      { key: 'patient_document', label: 'Documento', placeholder: 'DNI / CUIL' },
      { key: 'employer_name', label: 'Empresa', placeholder: 'Empresa cliente' },
      {
        key: 'exam_type',
        label: 'Tipo de examen',
        type: 'select',
        options: Object.entries(occupationalExamTypeLabels).map(([value, label]) => ({ value, label })),
      },
      {
        key: 'status',
        label: 'Estado',
        type: 'select',
        options: Object.entries(occupationalExamStatusLabels).map(([value, label]) => ({ value, label })),
      },
      { key: 'scheduled_at', label: 'Fecha programada', type: 'datetime-local' },
      { key: 'result', label: 'Resultado / apto', placeholder: 'Apto, no apto, observado...', fullWidth: true },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    rowActions: [
      {
        id: 'schedule',
        label: 'Agendar',
        kind: 'secondary',
        isVisible: (row, ctx) => !ctx.archived && row.status !== 'scheduled' && row.status !== 'completed',
        onClick: async (row) => {
          await updateOccupationalHealthExam(row.id, { status: 'scheduled' });
        },
      },
      {
        id: 'complete',
        label: 'Completar',
        kind: 'success',
        isVisible: (row, ctx) => !ctx.archived && row.status !== 'completed',
        onClick: async (row) => {
          await updateOccupationalHealthExam(row.id, { status: 'completed', completed_at: new Date().toISOString() });
        },
      },
    ],
    searchText: (row) =>
      [
        row.patient_name,
        row.patient_document,
        row.employer_name,
        occupationalExamTypeLabels[row.exam_type],
        occupationalExamStatusLabels[row.status],
        row.result,
        row.notes,
      ].filter(Boolean).join(' '),
    toFormValues: (row) => ({
      patient_name: row?.patient_name ?? '',
      patient_document: row?.patient_document ?? '',
      employer_name: row?.employer_name ?? '',
      exam_type: row?.exam_type ?? 'pre_employment',
      status: row?.status ?? 'pending',
      scheduled_at: toDateTimeInput(row?.scheduled_at ?? undefined),
      result: row?.result ?? '',
      notes: row?.notes ?? '',
    }) as CrudFormValues,
    toBody: toExamBody,
    isValid: (values) => asString(values.patient_name).trim().length >= 2,
    featureFlags: { standardAnnotations: false, standardMedia: false, tagPills: false },
  };
}
