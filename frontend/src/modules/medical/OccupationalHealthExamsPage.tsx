import { FormEvent, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { PageLayout } from '../../components/PageLayout';
import {
  archiveOccupationalHealthExam,
  createOccupationalHealthExam,
  listOccupationalHealthExams,
  updateOccupationalHealthExam,
} from '../../lib/medicalApi';
import type { OccupationalExamStatus, OccupationalExamType } from '../../lib/medicalTypes';

const examTypeLabels: Record<OccupationalExamType, string> = {
  pre_employment: 'Preocupacional',
  periodic: 'Periódico',
  return_to_work: 'Reintegro',
  exit: 'Egreso',
  other: 'Otro',
};

const statusLabels: Record<OccupationalExamStatus, string> = {
  pending: 'Pendiente',
  scheduled: 'Agendado',
  completed: 'Completo',
  cancelled: 'Cancelado',
};

type FormState = {
  patient_name: string;
  patient_document: string;
  employer_name: string;
  exam_type: OccupationalExamType;
  status: OccupationalExamStatus;
  scheduled_at: string;
  notes: string;
};

const initialForm: FormState = {
  patient_name: '',
  patient_document: '',
  employer_name: '',
  exam_type: 'pre_employment',
  status: 'pending',
  scheduled_at: '',
  notes: '',
};

export function OccupationalHealthExamsPage() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState('');
  const [form, setForm] = useState<FormState>(initialForm);
  const [error, setError] = useState('');

  const examsQuery = useQuery({
    queryKey: ['medical', 'occupational-health', 'exams', search, status],
    queryFn: () => listOccupationalHealthExams({ search, status }),
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['medical', 'occupational-health', 'exams'] });

  const createMutation = useMutation({
    mutationFn: createOccupationalHealthExam,
    onSuccess: async () => {
      setForm(initialForm);
      await invalidate();
    },
  });

  const statusMutation = useMutation({
    mutationFn: ({ id, nextStatus }: { id: string; nextStatus: OccupationalExamStatus }) =>
      updateOccupationalHealthExam(id, {
        status: nextStatus,
        completed_at: nextStatus === 'completed' ? new Date().toISOString() : null,
      }),
    onSuccess: invalidate,
  });

  const archiveMutation = useMutation({
    mutationFn: archiveOccupationalHealthExam,
    onSuccess: invalidate,
  });

  const busy = createMutation.isPending || statusMutation.isPending || archiveMutation.isPending;
  const items = useMemo(() => examsQuery.data?.items ?? [], [examsQuery.data?.items]);
  const total = examsQuery.data?.total ?? items.length;

  const summary = useMemo(() => {
    const pending = items.filter((item) => item.status === 'pending').length;
    const scheduled = items.filter((item) => item.status === 'scheduled').length;
    const completed = items.filter((item) => item.status === 'completed').length;
    return { pending, scheduled, completed };
  }, [items]);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError('');
    try {
      await createMutation.mutateAsync({
        ...form,
        scheduled_at: form.scheduled_at ? new Date(form.scheduled_at).toISOString() : null,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No se pudo crear el examen.');
    }
  }

  async function setExamStatus(id: string, nextStatus: OccupationalExamStatus) {
    setError('');
    try {
      await statusMutation.mutateAsync({ id, nextStatus });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No se pudo actualizar el estado.');
    }
  }

  async function archiveExam(id: string) {
    setError('');
    try {
      await archiveMutation.mutateAsync(id);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No se pudo archivar el examen.');
    }
  }

  return (
    <PageLayout title="Medicina laboral" lead="Exámenes y aptos laborales por trabajador.">
      <section className="crud-section-band">
        <div className="crud-toolbar">
          <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Buscar trabajador, DNI o empresa..." />
          <select value={status} onChange={(event) => setStatus(event.target.value)}>
            <option value="">Todos los estados</option>
            {Object.entries(statusLabels).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </div>

        <div className="dashboard-kpi-grid">
          <div className="kpi-card">
            <span>Total</span>
            <strong>{total}</strong>
            <small>exámenes</small>
          </div>
          <div className="kpi-card">
            <span>Pendientes</span>
            <strong>{summary.pending}</strong>
            <small>por gestionar</small>
          </div>
          <div className="kpi-card">
            <span>Agendados</span>
            <strong>{summary.scheduled}</strong>
            <small>con fecha</small>
          </div>
          <div className="kpi-card">
            <span>Completos</span>
            <strong>{summary.completed}</strong>
            <small>cerrados</small>
          </div>
        </div>
      </section>

      <section className="crud-section-band">
        <h2>Nuevo examen</h2>
        <form className="crud-form-grid" onSubmit={onSubmit}>
          <input required value={form.patient_name} onChange={(event) => setForm((current) => ({ ...current, patient_name: event.target.value }))} placeholder="Trabajador" />
          <input value={form.patient_document} onChange={(event) => setForm((current) => ({ ...current, patient_document: event.target.value }))} placeholder="Documento" />
          <input value={form.employer_name} onChange={(event) => setForm((current) => ({ ...current, employer_name: event.target.value }))} placeholder="Empresa" />
          <select value={form.exam_type} onChange={(event) => setForm((current) => ({ ...current, exam_type: event.target.value as OccupationalExamType }))}>
            {Object.entries(examTypeLabels).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
          <select value={form.status} onChange={(event) => setForm((current) => ({ ...current, status: event.target.value as OccupationalExamStatus }))}>
            {Object.entries(statusLabels).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
          <input type="datetime-local" value={form.scheduled_at} onChange={(event) => setForm((current) => ({ ...current, scheduled_at: event.target.value }))} />
          <input className="crud-form-grid__wide" value={form.notes} onChange={(event) => setForm((current) => ({ ...current, notes: event.target.value }))} placeholder="Notas" />
          <button type="submit" className="btn-primary" disabled={busy}>
            Crear examen
          </button>
        </form>
        {error ? <p className="alert alert-error">{error}</p> : null}
      </section>

      <section className="crud-section-band">
        <h2>Exámenes</h2>
        {examsQuery.isLoading ? <p>Cargando...</p> : null}
        {items.length === 0 && !examsQuery.isLoading ? <p>No hay exámenes para mostrar.</p> : null}
        {items.length > 0 ? (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Trabajador</th>
                  <th>Empresa</th>
                  <th>Tipo</th>
                  <th>Estado</th>
                  <th>Fecha</th>
                  <th>Acciones</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id}>
                    <td>
                      <strong>{item.patient_name}</strong>
                      <div className="text-muted">{item.patient_document || 'Sin documento'}</div>
                    </td>
                    <td>{item.employer_name || 'Sin empresa'}</td>
                    <td>{examTypeLabels[item.exam_type]}</td>
                    <td>{statusLabels[item.status]}</td>
                    <td>{item.scheduled_at ? new Date(item.scheduled_at).toLocaleString() : 'Sin fecha'}</td>
                    <td>
                      <div className="inline-actions">
                        <button type="button" className="btn-secondary" disabled={busy} onClick={() => void setExamStatus(item.id, 'scheduled')}>
                          Agendar
                        </button>
                        <button type="button" className="btn-secondary" disabled={busy} onClick={() => void setExamStatus(item.id, 'completed')}>
                          Completar
                        </button>
                        <button type="button" className="btn-secondary" disabled={busy} onClick={() => void archiveExam(item.id)}>
                          Archivar
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </section>
    </PageLayout>
  );
}
