import { DateTime } from "luxon";
import type { Booking, BookingAction, Resource } from "../domain/scheduling";
import { bookingStatusLabels, resourceLabel } from "../domain/scheduling";

type BookingDetailsProps = {
  booking: Booking | null;
  resources: Resource[];
  timezone: string;
  pending: boolean;
  canOperate: boolean;
  onClose: () => void;
  onEdit: (booking: Booking) => void;
  onReschedule: (booking: Booking) => void;
  onAction: (booking: Booking, action: BookingAction) => Promise<void>;
};

const allowedActions: Record<Booking["status"], BookingAction[]> = {
  held: ["confirm", "cancel"],
  pending_confirmation: ["confirm", "cancel"],
  confirmed: ["check-in", "cancel", "no-show"],
  checked_in: ["complete", "cancel"],
  completed: [],
  cancelled: [],
  rescheduled: [],
  no_show: [],
};

const actionLabels: Record<BookingAction, string> = {
  confirm: "Confirmar",
  cancel: "Cancelar",
  "check-in": "Registrar llegada",
  complete: "Completar",
  "no-show": "Marcar ausencia",
};

export function BookingDetails({
  booking,
  resources,
  timezone,
  pending,
  canOperate,
  onClose,
  onEdit,
  onReschedule,
  onAction,
}: BookingDetailsProps) {
  if (!booking) return null;
  const start = DateTime.fromISO(booking.start_at).setZone(timezone);
  const end = DateTime.fromISO(booking.end_at).setZone(timezone);
  const assigned = booking.allocations
    .map((allocation) => resources.find((resource) => resource.id === allocation.resource_id))
    .filter((resource): resource is Resource => Boolean(resource));
  const canReschedule = ["held", "pending_confirmation", "confirmed"].includes(booking.status);

  return (
    <aside className="booking-drawer" aria-label="Detalle del turno">
      <header>
        <div>
          <p className="eyebrow">Turno seleccionado</p>
          <h2>{booking.service_name}</h2>
        </div>
        <button type="button" className="icon-button" onClick={onClose} aria-label="Cerrar detalle">
          ×
        </button>
      </header>
      <div className={`status-pill status-pill--${booking.status}`}>{bookingStatusLabels[booking.status]}</div>
      <dl className="booking-facts">
        <div>
          <dt>Cuándo</dt>
          <dd>
            {start.toFormat("cccc d 'de' LLLL", { locale: "es" })}
            <strong>
              {start.toFormat("HH:mm")}–{end.toFormat("HH:mm")}
            </strong>
          </dd>
        </div>
        <div>
          <dt>Cliente</dt>
          <dd>{booking.customer_name || `Cliente · ${booking.party_id.slice(-6)}`}</dd>
        </div>
        <div>
          <dt>Valor</dt>
          <dd>
            {booking.currency} {booking.price}
          </dd>
        </div>
        <div>
          <dt>Participantes</dt>
          <dd>{booking.participants}</dd>
        </div>
        {booking.substate_code ? (
          <div>
            <dt>Subestado</dt>
            <dd>{booking.substate_code}</dd>
          </div>
        ) : null}
      </dl>
      {booking.notes ? (
        <section className="drawer-section">
          <h3>Nota interna</h3>
          <p>{booking.notes}</p>
        </section>
      ) : null}
      <section className="drawer-section">
        <h3>Recursos</h3>
        {assigned.length ? (
          <ul className="plain-list">
            {assigned.map((resource) => (
              <li key={resource.id}>{resourceLabel(resource)}</li>
            ))}
          </ul>
        ) : (
          <p className="muted">Sin recurso nominal asignado.</p>
        )}
      </section>
      <footer className="drawer-actions">
        {canOperate ? (
          <button type="button" className="button button--secondary" onClick={() => onEdit(booking)}>
            Editar
          </button>
        ) : null}
        {canOperate && canReschedule ? (
          <button type="button" className="button button--secondary" onClick={() => onReschedule(booking)}>
            Reprogramar
          </button>
        ) : null}
        {canOperate ? allowedActions[booking.status].map((action) => (
          <button
            key={action}
            type="button"
            className={action === "cancel" ? "button button--danger" : "button button--primary"}
            disabled={pending}
            onClick={() => void onAction(booking, action)}
          >
            {actionLabels[action]}
          </button>
        )) : (
          <p className="muted">Tenés acceso de sólo lectura.</p>
        )}
      </footer>
    </aside>
  );
}
