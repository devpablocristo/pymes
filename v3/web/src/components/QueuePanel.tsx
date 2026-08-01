import type { QueueTicket, Service } from "../domain/scheduling";

const queueStatusLabels: Record<QueueTicket["status"], string> = {
  waiting: "Esperando",
  called: "Llamado",
  serving: "En atención",
  completed: "Completado",
  no_show: "No respondió",
  cancelled: "Cancelado",
};

export function QueuePanel({
  tickets,
  services,
  pendingTicketId,
  canOperate,
  onAdvance,
}: {
  tickets: QueueTicket[];
  services: Service[];
  pendingTicketId: string | null;
  canOperate: boolean;
  onAdvance: (ticket: QueueTicket, status: "called" | "serving" | "completed" | "no_show" | "cancelled") => void;
}) {
  const active = tickets
    .filter((ticket) => !["completed", "cancelled", "no_show"].includes(ticket.status))
    .sort((a, b) => (b.priority ?? 0) - (a.priority ?? 0) || a.number - b.number);

  return (
    <div className="queue-board">
      <header className="section-heading">
        <div>
          <p className="eyebrow">Atención sin horario</p>
          <h2>Cola de hoy</h2>
        </div>
        <span className="count-badge">{active.length} activos</span>
      </header>
      {active.length ? (
        <div className="queue-board__lanes">
          {(["waiting", "called", "serving"] as const).map((status) => (
            <section key={status} className="queue-lane">
              <header>
                <h3>{queueStatusLabels[status]}</h3>
                <span>{active.filter((ticket) => ticket.status === status).length}</span>
              </header>
              <div className="queue-lane__items">
                {active
                  .filter((ticket) => ticket.status === status)
                  .map((ticket) => (
                    <article key={ticket.id} className="queue-ticket">
                      <span className="queue-ticket__number">{String(ticket.number).padStart(2, "0")}</span>
                      <div>
                        <strong>Cliente · {ticket.party_id.slice(-6)}</strong>
                        <span>{services.find((service) => service.id === ticket.service_id)?.name ?? "Servicio"}</span>
                      </div>
                      {canOperate ? <div className="queue-ticket__actions">
                        {status === "waiting" ? (
                          <button
                            type="button"
                            disabled={pendingTicketId === ticket.id}
                            onClick={() => onAdvance(ticket, "called")}
                          >
                            Llamar
                          </button>
                        ) : null}
                        {status === "called" ? (
                          <button
                            type="button"
                            disabled={pendingTicketId === ticket.id}
                            onClick={() => onAdvance(ticket, "serving")}
                          >
                            Atender
                          </button>
                        ) : null}
                        {status === "serving" ? (
                          <button
                            type="button"
                            disabled={pendingTicketId === ticket.id}
                            onClick={() => onAdvance(ticket, "completed")}
                          >
                            Completar
                          </button>
                        ) : null}
                        <button
                          type="button"
                          className="quiet-link"
                          disabled={pendingTicketId === ticket.id}
                          onClick={() => onAdvance(ticket, "no_show")}
                        >
                          No respondió
                        </button>
                      </div> : null}
                    </article>
                  ))}
              </div>
            </section>
          ))}
        </div>
      ) : (
        <div className="empty-state">
          <h3>La cola está libre</h3>
          <p>Los tickets nuevos aparecerán ordenados por prioridad y número.</p>
        </div>
      )}
    </div>
  );
}
