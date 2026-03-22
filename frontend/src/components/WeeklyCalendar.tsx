import { useEffect, useMemo, useState, type CSSProperties } from 'react';
import { useI18n } from '../lib/i18n';
import { vocab } from '../lib/vocabulary';
import { apiRequest } from '../lib/api';

type CalendarEvent = {
  id: string;
  title: string;
  day: number; // 0=Mon … 6=Sun
  startHour: number;
  durationHours: number;
  color?: string;
};

const HOUR_START = 7;
const HOUR_END = 22;
const HOURS = Array.from({ length: HOUR_END - HOUR_START }, (_, i) => HOUR_START + i);
const EVENT_COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#8b5cf6', '#ec4899', '#06b6d4', '#f97316'];

function getWeekDates(offset: number): Date[] {
  const now = new Date();
  const monday = new Date(now);
  monday.setDate(now.getDate() - ((now.getDay() + 6) % 7) + offset * 7);
  return Array.from({ length: 7 }, (_, i) => {
    const d = new Date(monday);
    d.setDate(monday.getDate() + i);
    return d;
  });
}

function isToday(d: Date): boolean {
  const now = new Date();
  return d.getDate() === now.getDate() && d.getMonth() === now.getMonth() && d.getFullYear() === now.getFullYear();
}

function isoDate(d: Date): string {
  return d.toISOString().slice(0, 10);
}

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

function toCalendarEvents(raw: RawAppointment[], untitledLabel: string): CalendarEvent[] {
  return raw
    .map((apt, i) => {
      const dateStr = apt.scheduled_at ?? apt.start_time ?? apt.started_at;
      if (!dateStr) return null;
      const date = new Date(dateStr);
      const dayOfWeek = (date.getDay() + 6) % 7;
      const hour = date.getHours() + date.getMinutes() / 60;
      return {
        id: apt.id,
        title: apt.title ?? apt.customer_name ?? apt.display_name ?? untitledLabel,
        day: dayOfWeek,
        startHour: hour,
        durationHours: (apt.duration_minutes ?? 60) / 60,
        color: EVENT_COLORS[i % EVENT_COLORS.length],
      } as CalendarEvent;
    })
    .filter((e): e is CalendarEvent => e !== null && e.startHour >= HOUR_START && e.startHour < HOUR_END);
}

// Deterministic demo data seeded by week dates — franjas de 1 h separadas ≥2 h para no superponer en la grilla.
function generateDemoEvents(dates: Date[]): CalendarEvent[] {
  const names = ['María García', 'Juan Pérez', 'Ana López', 'Carlos Ruiz', 'Laura Díaz', 'Pedro Sánchez', 'Sofía Torres'];
  const events: CalendarEvent[] = [];
  let id = 0;
  const seed = dates[0].getDate();

  for (let day = 0; day < 6; day++) {
    const count = 2 + ((seed + day) % 3);
    // Desfase 0–1 h según semana/día; dentro del día solo saltos de 2 h (sin solapes).
    const stagger = (seed + day * 3) % 2;
    for (let j = 0; j < count; j++) {
      const hour = 8 + stagger + j * 2;
      if (hour + 1 > HOUR_END) break;

      const nameIdx = (day * 3 + j + seed) % names.length;
      events.push({
        id: String(++id),
        title: names[nameIdx],
        day,
        startHour: hour,
        durationHours: 1,
        color: EVENT_COLORS[nameIdx % EVENT_COLORS.length],
      });
    }
  }
  return events;
}

export function WeeklyCalendar() {
  const { language, t, localizeText } = useI18n();
  const dayLabels = useMemo(
    () => [
      t('calendar.day.mon'),
      t('calendar.day.tue'),
      t('calendar.day.wed'),
      t('calendar.day.thu'),
      t('calendar.day.fri'),
      t('calendar.day.sat'),
      t('calendar.day.sun'),
    ],
    [t],
  );
  const [weekOffset, setWeekOffset] = useState(0);
  const dates = useMemo(() => getWeekDates(weekOffset), [weekOffset]);
  const [events, setEvents] = useState<CalendarEvent[]>([]);
  const [isDemo, setIsDemo] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);

    const from = isoDate(dates[0]);
    const to = isoDate(dates[6]);

    apiRequest<{ items?: RawAppointment[] }>(`/v1/appointments?from=${from}&to=${to}`)
      .then((data) => {
        if (cancelled) return;
        const items = data.items ?? (Array.isArray(data) ? data : []);
        if (items.length > 0) {
          setEvents(toCalendarEvents(items as RawAppointment[], t('calendar.event.untitled')));
          setIsDemo(false);
        } else {
          setEvents(generateDemoEvents(dates));
          setIsDemo(true);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setEvents(generateDemoEvents(dates));
          setIsDemo(true);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => { cancelled = true; };
  }, [dates, t]);

  const todayDayOfWeek = (new Date().getDay() + 6) % 7;
  const todayCount = events.filter((e) => e.day === todayDayOfWeek).length;
  const weekCount = events.length;

  return (
    <div className="weekly-calendar">
      <div className="weekly-calendar-toolbar">
        <div className="weekly-calendar-nav">
          <button type="button" onClick={() => setWeekOffset((o) => o - 1)} className="weekly-cal-btn">&larr;</button>
          <button type="button" onClick={() => setWeekOffset(0)} className="weekly-cal-btn weekly-cal-btn-today">{t('calendar.button.today')}</button>
          <button type="button" onClick={() => setWeekOffset((o) => o + 1)} className="weekly-cal-btn">&rarr;</button>
        </div>
        <div className="weekly-calendar-title">
          <h2>
            {dates[0].toLocaleDateString(language === 'en' ? 'en-US' : 'es-AR', { month: 'long', year: 'numeric' })}
          </h2>
        </div>
        <div className="weekly-calendar-stats">
          <span className="weekly-cal-stat">
            <strong>{todayCount}</strong> {t('calendar.stat.today')}
          </span>
          <span className="weekly-cal-stat">
            <strong>{weekCount}</strong> {t('calendar.stat.week')}
          </span>
        </div>
      </div>

      {loading ? (
        <div className="spinner" />
      ) : (
        <div className="weekly-calendar-grid">
          <div className="weekly-cal-corner" />
          {dates.map((date, i) => (
            <div key={i} className={`weekly-cal-day-header${isToday(date) ? ' today' : ''}`}>
              <span className="weekly-cal-day-name">{dayLabels[i]}</span>
              <span className={`weekly-cal-day-number${isToday(date) ? ' today' : ''}`}>{date.getDate()}</span>
            </div>
          ))}

          {HOURS.map((hour) => (
            <div key={hour} className="weekly-cal-row" style={{ gridRow: `${hour - HOUR_START + 2}` }}>
              <div className="weekly-cal-time">
                {hour.toString().padStart(2, '0')}:00
              </div>
              {Array.from({ length: 7 }, (_, day) => (
                <div key={day} className={`weekly-cal-cell${isToday(dates[day]) ? ' today-col' : ''}`} />
              ))}
            </div>
          ))}

          {events.map((event) => {
            const topPx = (event.startHour - HOUR_START) * 60;
            const heightPx = Math.max(event.durationHours * 60 - 2, 24);

            return (
              <div
                key={event.id}
                className="weekly-cal-event"
                style={
                  {
                    '--cal-day': event.day,
                    '--cal-top': `${topPx}px`,
                    '--cal-height': `${heightPx}px`,
                    backgroundColor: event.color ?? '#3b82f6',
                  } as CSSProperties
                }
              >
                <span className="weekly-cal-event-time">
                  {Math.floor(event.startHour).toString().padStart(2, '0')}:
                  {String(Math.round((event.startHour % 1) * 60)).padStart(2, '0')}
                </span>
                <span className="weekly-cal-event-title">{event.title}</span>
              </div>
            );
          })}
        </div>
      )}

      {isDemo && !loading && (
        <div className="weekly-calendar-footer">
          <small>
            {t('calendar.demo', { appointments: localizeText(vocab('Turnos')) })}
          </small>
        </div>
      )}
    </div>
  );
}
