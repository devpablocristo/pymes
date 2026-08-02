import { DateTime } from "luxon";
import type { Booking } from "../domain/scheduling";
import { bookingStatusLabels } from "../domain/scheduling";

export function DayRail({
  bookings,
  timezone,
  onSelect,
}: {
  bookings: Booking[];
  timezone: string;
  onSelect: (booking: Booking) => void;
}) {
  const today = DateTime.now().setZone(timezone);
  const dayBookings = bookings
    .filter((booking) => DateTime.fromISO(booking.start_at).setZone(timezone).hasSame(today, "day"))
    .sort((a, b) => a.start_at.localeCompare(b.start_at));

  return (
    <aside className="day-rail" aria-label="Jornada de hoy">
      <header>
        <p className="eyebrow">Hoy · {today.toFormat("ccc d", { locale: "es" })}</p>
        <h2>Regla de jornada</h2>
        <span>{dayBookings.length} turnos</span>
      </header>
      <div className="day-rail__line" aria-hidden="true" />
      {dayBookings.length === 0 ? (
        <div className="day-rail__empty">
          <span>08:00</span>
          <p>La jornada está libre. Creá un turno desde el calendario.</p>
        </div>
      ) : (
        <ol>
          {dayBookings.map((booking) => {
            const start = DateTime.fromISO(booking.start_at).setZone(timezone);
            return (
              <li key={booking.id}>
                <button type="button" onClick={() => onSelect(booking)}>
                  <time>{start.toFormat("HH:mm")}</time>
                  <span>
                    <strong>{booking.service_name}</strong>
                    <small>{bookingStatusLabels[booking.status]}</small>
                  </span>
                </button>
              </li>
            );
          })}
        </ol>
      )}
    </aside>
  );
}
