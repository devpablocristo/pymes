/**
 * Calendario estilo Google Calendar — conectado a la API /v1/appointments.
 * CRUD real: crear, mover (drag), resize, eliminar.
 * FullCalendar con toolbar custom y estilos nativos.
 */
import { useState, useCallback, useRef, useEffect } from 'react';
import { IconClose } from '../components/Icons';
import FullCalendar from '@fullcalendar/react';
import dayGridPlugin from '@fullcalendar/daygrid';
import timeGridPlugin from '@fullcalendar/timegrid';
import interactionPlugin from '@fullcalendar/interaction';
import type { EventInput, DateSelectArg, EventClickArg, EventApi, EventDropArg } from '@fullcalendar/core';
import type { EventResizeDoneArg } from '@fullcalendar/interaction';
import { apiRequest } from '../lib/api';
import './CalendarPage.css';

// ─── Tipos API ───

type ApiAppointment = {
  id: string;
  title: string;
  customer_name: string;
  customer_phone?: string;
  description?: string;
  status: string;
  start_at: string;
  end_at: string;
  duration: number;
  location?: string;
  assigned_to?: string;
  color: string;
  notes?: string;
};

const COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899', '#06b6d4', '#64748b'];

function apiToEvent(a: ApiAppointment): EventInput {
  return {
    id: a.id,
    title: a.title || a.customer_name || 'Sin título',
    start: a.start_at,
    end: a.end_at,
    color: a.color || '#3b82f6',
    extendedProps: { appointment: a },
  };
}

// ─── Modal ───

type ModalMode = 'create' | 'edit';

type ModalState = {
  open: boolean;
  mode: ModalMode;
  id?: string;
  title: string;
  customerName: string;
  customerPhone: string;
  start: string;
  end: string;
  color: string;
  description: string;
  notes: string;
  location: string;
  assignedTo: string;
  allDay: boolean;
};

const emptyModal: ModalState = {
  open: false, mode: 'create', title: '', customerName: '', customerPhone: '',
  start: '', end: '', color: '#3b82f6', description: '', notes: '', location: '', assignedTo: '', allDay: false,
};

function EventModal({
  state, saving, onSave, onDelete, onClose,
}: {
  state: ModalState;
  saving: boolean;
  onSave: (s: ModalState) => void;
  onDelete?: () => void;
  onClose: () => void;
}) {
  const [form, setForm] = useState(state);
  useEffect(() => setForm(state), [state]);

  const set = (k: keyof ModalState) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
    setForm((p) => ({ ...p, [k]: e.target.value }));

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.title.trim() && !form.customerName.trim()) return;
    onSave(form);
  };

  return (
    <div className="gcal__backdrop" onClick={onClose}>
      <form className="gcal__modal" onClick={(e) => e.stopPropagation()} onSubmit={handleSubmit}>
        <div className="gcal__modal-header">
          <h3 className="gcal__modal-title">{state.mode === 'edit' ? 'Editar turno' : 'Nuevo turno'}</h3>
          <button type="button" className="gcal__modal-close" onClick={onClose}><IconClose /></button>
        </div>
        <div className="gcal__modal-body">
          <div className="form-group">
            <label htmlFor="gcal-title">Título</label>
            <input id="gcal-title" value={form.title} onChange={set('title')} placeholder="Consulta, control, etc." autoFocus />
          </div>
          <div className="form-row">
            <div className="form-group grow">
              <label htmlFor="gcal-customer">Cliente</label>
              <input id="gcal-customer" value={form.customerName} onChange={set('customerName')} required />
            </div>
            <div className="form-group grow">
              <label htmlFor="gcal-phone">Teléfono</label>
              <input id="gcal-phone" value={form.customerPhone} onChange={set('customerPhone')} />
            </div>
          </div>
          <div className="form-row">
            <div className="form-group grow">
              <label htmlFor="gcal-start">Inicio</label>
              <input id="gcal-start" type="datetime-local" value={form.start} onChange={set('start')} required />
            </div>
            <div className="form-group grow">
              <label htmlFor="gcal-end">Fin</label>
              <input id="gcal-end" type="datetime-local" value={form.end} onChange={set('end')} />
            </div>
          </div>
          <div className="form-row">
            <div className="form-group grow">
              <label htmlFor="gcal-assigned">Asignado a</label>
              <input id="gcal-assigned" value={form.assignedTo} onChange={set('assignedTo')} />
            </div>
            <div className="form-group grow">
              <label htmlFor="gcal-location">Ubicación</label>
              <input id="gcal-location" value={form.location} onChange={set('location')} />
            </div>
          </div>
          <div className="form-group">
            <label htmlFor="gcal-notes">Notas</label>
            <textarea id="gcal-notes" rows={2} value={form.notes} onChange={set('notes')} />
          </div>
          <div className="form-group">
            <label>Color</label>
            <div className="gcal__color-row">
              {COLORS.map((c) => (
                <button key={c} type="button" className={`gcal__color-dot ${form.color === c ? 'gcal__color-dot--active' : ''}`} style={{ background: c }} onClick={() => setForm((p) => ({ ...p, color: c }))} />
              ))}
            </div>
          </div>
        </div>
        <div className="gcal__modal-footer">
          {state.mode === 'edit' && onDelete && (
            <button type="button" className="btn-danger btn-sm" onClick={onDelete} disabled={saving}>Eliminar</button>
          )}
          <div style={{ flex: 1 }} />
          <button type="button" className="btn-secondary btn-sm" onClick={onClose}>Cancelar</button>
          <button type="submit" className="btn-primary btn-sm" disabled={saving}>
            {saving ? 'Guardando…' : state.mode === 'edit' ? 'Guardar' : 'Crear'}
          </button>
        </div>
      </form>
    </div>
  );
}

// ─── Página ───

type ViewType = 'dayGridMonth' | 'timeGridWeek' | 'timeGridDay';

function toLocalDatetime(iso: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  const off = d.getTimezoneOffset();
  const local = new Date(d.getTime() - off * 60000);
  return local.toISOString().slice(0, 16);
}

export function CalendarPage() {
  const calRef = useRef<FullCalendar>(null);
  const [modal, setModal] = useState<ModalState>(emptyModal);
  const [saving, setSaving] = useState(false);
  const [view, setView] = useState<ViewType>('timeGridWeek');
  const [titleText, setTitleText] = useState('');
  const [events, setEvents] = useState<EventInput[]>([]);
  const [loaded, setLoaded] = useState(false);

  // Cargar eventos de la API
  const loadEvents = useCallback(async () => {
    try {
      const data = await apiRequest<{ items?: ApiAppointment[] }>('/v1/appointments?limit=200');
      const items = data.items ?? (Array.isArray(data) ? data : []);
      setEvents((items as ApiAppointment[]).map(apiToEvent));
    } catch {
      // Sin datos — quedará vacío
    } finally {
      setLoaded(true);
    }
  }, []);

  useEffect(() => { void loadEvents(); }, [loadEvents]);

  // Toolbar custom
  const updateTitle = useCallback(() => {
    const api = calRef.current?.getApi();
    if (api) setTitleText(api.view.title);
  }, []);

  useEffect(() => { updateTitle(); }, [view, updateTitle]);

  const nav = (action: 'prev' | 'next' | 'today') => {
    const api = calRef.current?.getApi();
    if (!api) return;
    if (action === 'prev') api.prev();
    else if (action === 'next') api.next();
    else api.today();
    updateTitle();
  };

  const changeView = (v: ViewType) => {
    const api = calRef.current?.getApi();
    if (!api) return;
    api.changeView(v);
    setView(v);
    updateTitle();
  };

  // Crear evento al seleccionar rango
  const handleSelect = useCallback((info: DateSelectArg) => {
    info.view.calendar.unselect();
    setModal({
      open: true, mode: 'create', title: '', customerName: '', customerPhone: '',
      start: toLocalDatetime(info.startStr), end: toLocalDatetime(info.endStr),
      color: '#3b82f6', description: '', notes: '', location: '', assignedTo: '', allDay: info.allDay,
    });
  }, []);

  // Editar evento al clickear
  const handleEventClick = useCallback((info: EventClickArg) => {
    const a = info.event.extendedProps.appointment as ApiAppointment | undefined;
    setModal({
      open: true, mode: 'edit', id: info.event.id,
      title: a?.title ?? info.event.title, customerName: a?.customer_name ?? '',
      customerPhone: a?.customer_phone ?? '',
      start: toLocalDatetime(info.event.startStr), end: toLocalDatetime(info.event.endStr ?? info.event.startStr),
      color: info.event.backgroundColor || '#3b82f6',
      description: a?.description ?? '', notes: a?.notes ?? '',
      location: a?.location ?? '', assignedTo: a?.assigned_to ?? '', allDay: info.event.allDay,
    });
  }, []);

  // Drag-to-move
  const handleEventDrop = useCallback(async (info: EventDropArg) => {
    setSaving(true);
    try {
      await apiRequest(`/v1/appointments/${info.event.id}`, {
        method: 'PUT',
        body: { start_at: info.event.startStr, end_at: info.event.endStr ?? info.event.startStr },
      });
    } catch {
      info.revert();
    } finally {
      setSaving(false);
    }
  }, []);

  // Drag-to-resize
  const handleEventResize = useCallback(async (info: EventResizeDoneArg) => {
    setSaving(true);
    try {
      await apiRequest(`/v1/appointments/${info.event.id}`, {
        method: 'PUT',
        body: { start_at: info.event.startStr, end_at: info.event.endStr ?? info.event.startStr },
      });
    } catch {
      info.revert();
    } finally {
      setSaving(false);
    }
  }, []);

  // Guardar (crear o editar)
  const handleSave = useCallback(async (form: ModalState) => {
    setSaving(true);
    try {
      const body = {
        title: form.title.trim() || form.customerName.trim(),
        customer_name: form.customerName.trim(),
        customer_phone: form.customerPhone.trim() || undefined,
        start_at: new Date(form.start).toISOString(),
        end_at: form.end ? new Date(form.end).toISOString() : undefined,
        color: form.color,
        description: form.description.trim() || undefined,
        notes: form.notes.trim() || undefined,
        location: form.location.trim() || undefined,
        assigned_to: form.assignedTo.trim() || undefined,
      };

      if (form.mode === 'edit' && form.id) {
        await apiRequest(`/v1/appointments/${form.id}`, { method: 'PUT', body });
      } else {
        await apiRequest('/v1/appointments', { method: 'POST', body });
      }
      setModal(emptyModal);
      await loadEvents();
    } catch (err) {
      window.alert(String(err));
    } finally {
      setSaving(false);
    }
  }, [loadEvents]);

  // Eliminar (archive)
  const handleDelete = useCallback(async () => {
    if (!modal.id || !window.confirm('¿Archivar este turno?')) return;
    setSaving(true);
    try {
      await apiRequest(`/v1/appointments/${modal.id}`, { method: 'DELETE' });
      setModal(emptyModal);
      await loadEvents();
    } catch (err) {
      window.alert(String(err));
    } finally {
      setSaving(false);
    }
  }, [modal.id, loadEvents]);

  return (
    <div className="gcal">
      {/* Toolbar estilo Google Calendar */}
      <div className="gcal__toolbar">
        <div className="gcal__toolbar-left">
          <button type="button" className="gcal__today-btn" onClick={() => nav('today')}>Hoy</button>
          <button type="button" className="gcal__nav-btn" onClick={() => nav('prev')}>‹</button>
          <button type="button" className="gcal__nav-btn" onClick={() => nav('next')}>›</button>
          <h2 className="gcal__title">{titleText}</h2>
        </div>
        <div className="gcal__toolbar-right">
          <div className="gcal__view-group">
            {([['timeGridDay', 'Día'], ['timeGridWeek', 'Semana'], ['dayGridMonth', 'Mes']] as const).map(([v, label]) => (
              <button key={v} type="button" className={`gcal__view-btn ${view === v ? 'gcal__view-btn--active' : ''}`} onClick={() => changeView(v)}>
                {label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Calendario */}
      <div className="gcal__body">
        {!loaded ? (
          <div style={{ padding: '3rem', textAlign: 'center' }}><div className="spinner" /></div>
        ) : (
          <FullCalendar
            ref={calRef}
            plugins={[dayGridPlugin, timeGridPlugin, interactionPlugin]}
            initialView={view}
            headerToolbar={false}
            editable
            selectable
            selectMirror
            dayMaxEvents
            weekends
            events={events}
            select={handleSelect}
            eventClick={handleEventClick}
            eventDrop={handleEventDrop}
            eventResize={handleEventResize}
            datesSet={updateTitle}
            locale="es"
            height="100%"
            slotMinTime="00:00:00"
            slotMaxTime="24:00:00"
            scrollTime="07:00:00"
            allDayText=""
            noEventsText=""
            nowIndicator
            slotDuration="00:30:00"
            eventTimeFormat={{ hour: '2-digit', minute: '2-digit', hour12: false }}
          />
        )}
      </div>

      {/* Modal */}
      {modal.open && (
        <EventModal
          state={modal}
          saving={saving}
          onSave={handleSave}
          onDelete={modal.mode === 'edit' ? handleDelete : undefined}
          onClose={() => setModal(emptyModal)}
        />
      )}

      {/* Indicador de guardado */}
      {saving && <div className="gcal__saving">Guardando…</div>}
    </div>
  );
}

export default CalendarPage;
