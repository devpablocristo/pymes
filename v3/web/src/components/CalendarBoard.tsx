import { CalendarSurface, type CalendarView } from "@devpablocristo/platform-calendar-board/next";
import "@devpablocristo/platform-calendar-board/styles.next.css";
import { formatSchedulingClock } from "@devpablocristo/platform-scheduling/next";
import "@devpablocristo/platform-scheduling/styles.next.css";
import type {
  DatesSetArg,
  EventClickArg,
  EventContentArg,
  EventDropArg,
  EventInput,
} from "@fullcalendar/core";
import type { DateSelectArg } from "@fullcalendar/core";
import type { EventResizeDoneArg } from "@fullcalendar/interaction";
import FullCalendar from "@fullcalendar/react";
import { useMemo, useRef, useState } from "react";
import type { AvailabilityBlock, Booking, DateRange } from "../domain/scheduling";
import { bookingStatusLabels } from "../domain/scheduling";

type CalendarBoardProps = {
  bookings: Booking[];
  blocks: AvailabilityBlock[];
  loaded: boolean;
  timezone: string;
  canOperate: boolean;
  selectedBookingId?: string | null;
  onRangeChange: (range: DateRange) => void;
  onCreateAt: (startAt: string, endAt: string) => void;
  onSelectBooking: (booking: Booking) => void;
  onReschedule: (booking: Booking, startAt: string, endAt: string) => Promise<void>;
  onMutationError: (error: unknown) => void;
};

const viewOptions = [
  { value: "timeGridDay", label: "Día" },
  { value: "timeGridWeek", label: "Semana" },
  { value: "dayGridMonth", label: "Mes" },
  { value: "listWeek", label: "Lista" },
] as const;

function eventClass(status: Booking["status"], selected: boolean): string[] {
  return [`booking-event`, `booking-event--${status}`, selected ? "booking-event--selected" : ""].filter(Boolean);
}

export function CalendarBoard({
  bookings,
  blocks,
  loaded,
  timezone,
  canOperate,
  selectedBookingId,
  onRangeChange,
  onCreateAt,
  onSelectBooking,
  onReschedule,
  onMutationError,
}: CalendarBoardProps) {
  const calendarRef = useRef<FullCalendar | null>(null);
  const [view, setView] = useState<CalendarView>("timeGridWeek");
  const [title, setTitle] = useState("");
  const events = useMemo<EventInput[]>(() => {
    const bookingEvents = bookings.map<EventInput>((booking) => ({
      id: booking.id,
      title: booking.service_name,
      start: booking.start_at,
      end: booking.end_at,
      editable:
        canOperate &&
        !["cancelled", "completed", "rescheduled", "no_show"].includes(
          booking.status,
        ),
      classNames: eventClass(booking.status, booking.id === selectedBookingId),
      extendedProps: { kind: "booking", booking },
    }));
    const blockEvents = blocks.map<EventInput>((block) => ({
      id: `block:${block.id}`,
      title: block.reason || "Bloqueado",
      start: block.start_at,
      end: block.end_at,
      display: "background",
      classNames: ["availability-block", `availability-block--${block.kind}`],
      editable: false,
      extendedProps: { kind: "block" },
    }));
    return [...bookingEvents, ...blockEvents];
  }, [blocks, bookings, canOperate, selectedBookingId]);

  function api() {
    return calendarRef.current?.getApi();
  }

  function changeView(next: CalendarView) {
    setView(next);
    api()?.changeView(next);
  }

  function datesSet(arg: DatesSetArg) {
    setTitle(arg.view.title);
    onRangeChange({ from: arg.start.toISOString(), until: arg.end.toISOString() });
  }

  function selectRange(arg: DateSelectArg) {
    onCreateAt(arg.start.toISOString(), arg.end.toISOString());
    api()?.unselect();
  }

  function eventClick(arg: EventClickArg) {
    const booking = arg.event.extendedProps.booking as Booking | undefined;
    if (booking) {
      onSelectBooking(booking);
    }
  }

  async function persistMove(arg: EventDropArg | EventResizeDoneArg) {
    const booking = arg.event.extendedProps.booking as Booking | undefined;
    const start = arg.event.start;
    const end = arg.event.end;
    if (!booking || !start || !end) {
      arg.revert();
      return;
    }
    try {
      await onReschedule(booking, start.toISOString(), end.toISOString());
    } catch (error) {
      arg.revert();
      onMutationError(error);
    }
  }

  function renderEvent(arg: EventContentArg) {
    const booking = arg.event.extendedProps.booking as Booking | undefined;
    if (!booking) {
      return <span>{arg.event.title}</span>;
    }
    return (
      <div className="booking-event__content">
        <span className="booking-event__time">{formatSchedulingClock(booking.start_at, "es-AR")}</span>
        <strong>{booking.service_name}</strong>
        <span>{bookingStatusLabels[booking.status]}</span>
      </div>
    );
  }

  return (
    <CalendarSurface
      calendarRef={calendarRef}
      view={view}
      title={title}
      loaded={loaded}
      onToday={() => api()?.today()}
      onPrev={() => api()?.prev()}
      onNext={() => api()?.next()}
      onViewChange={changeView}
      viewOptions={viewOptions}
      locale="es"
      timeZone={timezone}
      slotMinTime="07:00:00"
      slotMaxTime="21:00:00"
      scrollTime="08:00:00"
      slotDuration="00:15:00"
      snapDuration="00:15:00"
      events={events}
      editable={canOperate}
      selectable={canOperate}
      eventDurationEditable={canOperate}
      onDatesSet={datesSet}
      onSelect={selectRange}
      onEventClick={eventClick}
      onEventDrop={(arg) => void persistMove(arg)}
      onEventResize={(arg) => void persistMove(arg)}
      eventContent={renderEvent}
      businessHours={{
        daysOfWeek: [1, 2, 3, 4, 5, 6],
        startTime: "08:00",
        endTime: "20:00",
      }}
      className="pymes-calendar-board"
      loadingFallback={<div className="calendar-loading" aria-live="polite">Cargando agenda…</div>}
    />
  );
}
