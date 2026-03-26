/**
 * Calendario unificado — combina lo mejor de WeeklyCalendar (API real, demo, stats)
 * con FullCalendar (mes/semana/día, drag & drop, creación de eventos).
 */
import { useState, useCallback, useRef, useEffect, useMemo } from 'react';
import FullCalendar from '@fullcalendar/react';
import dayGridPlugin from '@fullcalendar/daygrid';
import timeGridPlugin from '@fullcalendar/timegrid';
import interactionPlugin from '@fullcalendar/interaction';
import type { EventInput, DateSelectArg, EventClickArg, EventApi } from '@fullcalendar/core';
import { apiRequest } from '../lib/api';
import './CalendarDemoPage.css';

// ─── Colores ───

const EVENT_COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899'];

function colorFromId(id: string): string {
  let h = 0;
  for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) | 0;
  return EVENT_COLORS[Math.abs(h) % EVENT_COLORS.length];
}

let nextId = 500;
function uid() {
  nextId += 1;
  return String(nextId);
}

// ─── API → EventInput ───

type RawAppointment = {
  id: string;
  title?: string;
  customer_name?: string;
  display_name?: string;
  scheduled_at?: string;
  start_time?: string;
  started_at?: string;
  duration_minutes?: number;
  status?: string;
};

function appointmentsToEvents(items: RawAppointment[]): EventInput[] {
  return items
    .map((apt) => {
      const dateStr = apt.scheduled_at ?? apt.start_time ?? apt.started_at;
      if (!dateStr) return null;
      const start = new Date(dateStr);
      const mins = apt.duration_minutes ?? 60;
      const end = new Date(start.getTime() + mins * 60_000);
      const title = apt.title ?? apt.customer_name ?? apt.display_name ?? 'Sin título';
      return {
        id: apt.id,
        title,
        start: start.toISOString(),
        end: end.toISOString(),
        color: colorFromId(apt.id),
      } as EventInput;
    })
    .filter((e): e is EventInput => e !== null);
}

// ─── Demo data ───

function addDays(dateStr: string, days: number): string {
  const d = new Date(dateStr);
  d.setDate(d.getDate() + days);
  return d.toISOString().slice(0, 10);
}

function generateDemoEvents(): EventInput[] {
  const today = new Date().toISOString().slice(0, 10);
  const names = ['María García', 'Juan Pérez', 'Ana López', 'Carlos Ruiz', 'Laura Díaz', 'Pedro Sánchez', 'Sofía Torres'];
  const events: EventInput[] = [];
  const seed = new Date().getDate();

  for (let dayOffset = -2; dayOffset <= 5; dayOffset++) {
    const date = addDays(today, dayOffset);
    const count = 2 + ((seed + Math.abs(dayOffset)) % 3);
    const stagger = (seed + dayOffset * 3) % 2;
    for (let j = 0; j < count; j++) {
      const hour = 8 + stagger + j * 2;
      if (hour >= 20) break;
      const nameIdx = (Math.abs(dayOffset) * 3 + j + seed) % names.length;
      const id = uid();
      events.push({
        id,
        title: names[nameIdx],
        start: `${date}T${String(hour).padStart(2, '0')}:00:00`,
        end: `${date}T${String(hour + 1).padStart(2, '0')}:00:00`,
        color: colorFromId(id),
      });
    }
  }
  return events;
}

// ─── Modal ───

type ModalState = {
  open: boolean;
  title: string;
  start: string;
  end: string;
  allDay: boolean;
  color: string;
};

const emptyModal: ModalState = { open: false, title: '', start: '', end: '', allDay: false, color: '#3b82f6' };

function EventModal({
  state,
  onSave,
  onClose,
}: {
  state: ModalState;
  onSave: (title: string, color: string) => void;
  onClose: () => void;
}) {
  const [title, setTitle] = useState(state.title);
  const [color, setColor] = useState(state.color);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim()) return;
    onSave(title.trim(), color);
  };

  const dateDisplay = state.allDay
    ? new Date(state.start).toLocaleDateString('es-AR', { day: '2-digit', month: 'short', year: 'numeric' })
    : `${new Date(state.start).toLocaleString('es-AR', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })} → ${new Date(state.end).toLocaleString('es-AR', { hour: '2-digit', minute: '2-digit' })}`;

  return (
    <div className="cal-demo__backdrop" onClick={onClose}>
      <form className="cal-demo__modal" onClick={(e) => e.stopPropagation()} onSubmit={handleSubmit}>
        <div className="cal-demo__modal-header">
          <h3 className="cal-demo__modal-title">Nuevo evento</h3>
          <button type="button" className="cal-demo__modal-close" onClick={onClose}>✕</button>
        </div>
        <div className="cal-demo__modal-body">
          <div className="form-group">
            <label htmlFor="cal-title">Título</label>
            <input id="cal-title" type="text" value={title} onChange={(e) => setTitle(e.target.value)} autoFocus required />
          </div>
          <div className="form-group">
            <label>Fecha</label>
            <input type="text" value={dateDisplay} disabled />
          </div>
          <div className="form-group">
            <label>Color</label>
            <div className="cal-demo__color-picker">
              {EVENT_COLORS.map((c) => (
                <button
                  key={c}
                  type="button"
                  className={`cal-demo__color-swatch ${color === c ? 'cal-demo__color-swatch--active' : ''}`}
                  style={{ background: c }}
                  onClick={() => setColor(c)}
                />
              ))}
            </div>
          </div>
        </div>
        <div className="cal-demo__modal-footer">
          <button type="button" className="btn-secondary btn-sm" onClick={onClose}>Cancelar</button>
          <button type="submit" className="btn-primary btn-sm">Crear</button>
        </div>
      </form>
    </div>
  );
}

// ─── Sidebar ───

function SidebarStats({ events }: { events: EventApi[] }) {
  const now = new Date();
  const todayStr = now.toISOString().slice(0, 10);

  const todayCount = events.filter((e) => e.start && e.start.toISOString().slice(0, 10) === todayStr).length;

  const weekEnd = new Date(now);
  weekEnd.setDate(weekEnd.getDate() + 7);
  const weekCount = events.filter((e) => {
    if (!e.start) return false;
    return e.start >= now && e.start <= weekEnd;
  }).length;

  return (
    <div className="cal-demo__stats">
      <div className="cal-demo__stat">
        <strong>{todayCount}</strong>
        <span>hoy</span>
      </div>
      <div className="cal-demo__stat">
        <strong>{weekCount}</strong>
        <span>esta semana</span>
      </div>
      <div className="cal-demo__stat">
        <strong>{events.length}</strong>
        <span>total</span>
      </div>
    </div>
  );
}

function SidebarEvents({ events, onDelete }: { events: EventApi[]; onDelete: (id: string) => void }) {
  const upcoming = events
    .filter((e) => {
      if (!e.start) return false;
      const diff = (e.start.getTime() - Date.now()) / (1000 * 60 * 60 * 24);
      return diff >= -1 && diff <= 7;
    })
    .sort((a, b) => (a.start?.getTime() ?? 0) - (b.start?.getTime() ?? 0))
    .slice(0, 12);

  return (
    <div className="card">
      <h4 className="cal-demo__events-title">Próximos ({upcoming.length})</h4>
      {upcoming.length === 0 && (
        <p style={{ fontSize: '0.82rem', color: 'var(--color-text-muted)' }}>Sin eventos esta semana</p>
      )}
      {upcoming.map((ev) => (
        <div key={ev.id} className="cal-demo__event-item">
          <span className="cal-demo__event-dot" style={{ background: ev.backgroundColor || '#3b82f6' }} />
          <div className="cal-demo__event-info">
            <span className="cal-demo__event-time">
              {ev.allDay
                ? ev.start?.toLocaleDateString('es-AR', { day: '2-digit', month: 'short' })
                : ev.start?.toLocaleString('es-AR', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })}
            </span>
            <div className="cal-demo__event-name">{ev.title}</div>
          </div>
          <button type="button" className="cal-demo__event-delete" onClick={() => onDelete(ev.id)} title="Eliminar">✕</button>
        </div>
      ))}
    </div>
  );
}

// ─── Página principal ───

export function CalendarDemoPage() {
  const calendarRef = useRef<FullCalendar>(null);
  const [modal, setModal] = useState<ModalState>(emptyModal);
  const [currentEvents, setCurrentEvents] = useState<EventApi[]>([]);
  const [initialEvents, setInitialEvents] = useState<EventInput[] | null>(null);
  const [isDemo, setIsDemo] = useState(false);
  const [loading, setLoading] = useState(true);

  // Intentar cargar turnos reales; si falla o está vacío, demo
  useEffect(() => {
    let cancelled = false;
    const now = new Date();
    const from = addDays(now.toISOString().slice(0, 10), -30);
    const to = addDays(now.toISOString().slice(0, 10), 60);

    apiRequest<{ items?: RawAppointment[] }>(`/v1/appointments?from=${from}&to=${to}`)
      .then((data) => {
        if (cancelled) return;
        const items = data.items ?? (Array.isArray(data) ? data : []);
        if (items.length > 0) {
          setInitialEvents(appointmentsToEvents(items as RawAppointment[]));
          setIsDemo(false);
        } else {
          setInitialEvents(generateDemoEvents());
          setIsDemo(true);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setInitialEvents(generateDemoEvents());
          setIsDemo(true);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => { cancelled = true; };
  }, []);

  const handleDateSelect = useCallback((selectInfo: DateSelectArg) => {
    setModal({
      open: true,
      title: '',
      start: selectInfo.startStr,
      end: selectInfo.endStr,
      allDay: selectInfo.allDay,
      color: '#3b82f6',
    });
    selectInfo.view.calendar.unselect();
  }, []);

  const handleEventClick = useCallback((clickInfo: EventClickArg) => {
    if (window.confirm(`Eliminar "${clickInfo.event.title}"?`)) {
      clickInfo.event.remove();
    }
  }, []);

  const handleEventsSet = useCallback((events: EventApi[]) => {
    setCurrentEvents(events);
  }, []);

  const handleSave = useCallback(
    (title: string, color: string) => {
      const calApi = calendarRef.current?.getApi();
      if (!calApi) return;
      calApi.addEvent({
        id: uid(),
        title,
        start: modal.start,
        end: modal.end,
        allDay: modal.allDay,
        color,
      });
      setModal(emptyModal);
    },
    [modal],
  );

  const handleDeleteFromSidebar = useCallback((id: string) => {
    const calApi = calendarRef.current?.getApi();
    if (!calApi) return;
    const ev = calApi.getEventById(id);
    if (ev) ev.remove();
  }, []);

  return (
    <div className="cal-demo">
      <div className="page-header">
        <h1>Calendario</h1>
        <p style={{ color: 'var(--color-text-secondary)', margin: 0, fontSize: '0.88rem' }}>
          Turnos y eventos — clickeá una fecha para crear, arrastrá para mover
          {isDemo && !loading && <span style={{ marginLeft: '0.5rem', opacity: 0.7 }}>(datos de ejemplo)</span>}
        </p>
      </div>

      {loading ? (
        <div className="spinner" />
      ) : (
        <div className="cal-demo__layout">
          <div className="cal-demo__sidebar">
            <div className="card">
              <button
                type="button"
                className="btn-primary btn-sm cal-demo__add-btn"
                onClick={() => {
                  const now = new Date();
                  const start = now.toISOString().slice(0, 16);
                  now.setHours(now.getHours() + 1);
                  const end = now.toISOString().slice(0, 16);
                  setModal({ open: true, title: '', start, end, allDay: false, color: '#3b82f6' });
                }}
              >
                + Nuevo evento
              </button>
            </div>
            <div className="card">
              <SidebarStats events={currentEvents} />
            </div>
            <SidebarEvents events={currentEvents} onDelete={handleDeleteFromSidebar} />
          </div>

          <div className="cal-demo__main">
            <div className="card">
              <FullCalendar
                ref={calendarRef}
                plugins={[dayGridPlugin, timeGridPlugin, interactionPlugin]}
                headerToolbar={{
                  left: 'title',
                  center: 'timeGridDay,timeGridWeek,dayGridMonth',
                  right: 'prev,next today',
                }}
                initialView="dayGridMonth"
                editable
                selectable
                selectMirror
                dayMaxEvents
                weekends
                initialEvents={initialEvents ?? []}
                select={handleDateSelect}
                eventClick={handleEventClick}
                eventsSet={handleEventsSet}
                locale="es"
                buttonText={{ today: 'Hoy', month: 'Mes', week: 'Semana', day: 'Día' }}
                height="auto"
                slotMinTime="07:00:00"
                slotMaxTime="22:00:00"
                allDayText="Todo el día"
                noEventsText="Sin eventos"
              />
            </div>
          </div>
        </div>
      )}

      {modal.open && <EventModal state={modal} onSave={handleSave} onClose={() => setModal(emptyModal)} />}
    </div>
  );
}

export default CalendarDemoPage;
