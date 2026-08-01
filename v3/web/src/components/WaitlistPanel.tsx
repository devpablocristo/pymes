import { DateTime } from "luxon";
import { type FormEvent, useState } from "react";
import type { Service, WaitlistEntry, WaitlistInput } from "../domain/scheduling";

export function WaitlistPanel({
  entries,
  branchId,
  services,
  timezone,
  pending,
  onCreate,
}: {
  entries: WaitlistEntry[];
  branchId: string;
  services: Service[];
  timezone: string;
  pending: boolean;
  onCreate: (input: WaitlistInput) => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [serviceId, setServiceId] = useState(services[0]?.id ?? "");
  const [day, setDay] = useState(DateTime.now().setZone(timezone).plus({ days: 1 }).toISODate() ?? "");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const from = DateTime.fromISO(day, { zone: timezone }).startOf("day");
    const customer: WaitlistInput["customer"] = { name };
    if (email) customer.email = email;
    await onCreate({
      branch_id: branchId,
      service_id: serviceId,
      customer,
      preferred_from: from.toUTC().toISO()!,
      preferred_until: from.endOf("day").toUTC().toISO()!,
      participants: 1,
    });
    setName("");
    setEmail("");
  }

  return (
    <div className="operation-panel operation-panel--waitlist">
      <section className="operation-panel__main">
        <header className="section-heading">
          <div>
            <p className="eyebrow">Demanda pendiente</p>
            <h2>Lista de espera</h2>
          </div>
          <span className="count-badge">{entries.filter((item) => item.status === "pending").length} esperando</span>
        </header>
        {entries.length ? (
          <div className="data-table" role="table" aria-label="Lista de espera">
            <div className="data-table__header" role="row">
              <span>Cliente</span>
              <span>Servicio</span>
              <span>Preferencia</span>
              <span>Estado</span>
            </div>
            {entries.map((entry) => (
              <div className="data-table__row" role="row" key={entry.id}>
                <span>
                  <strong>{entry.customer_name || `Cliente · ${entry.party_id.slice(-6)}`}</strong>
                  <small>{entry.customer_email || entry.customer_phone || "Sin contacto cargado"}</small>
                </span>
                <span>{services.find((service) => service.id === entry.service_id)?.name ?? "Servicio"}</span>
                <span>
                  {DateTime.fromISO(entry.preferred_from).setZone(timezone).toFormat("dd LLL", { locale: "es" })}
                </span>
                <span className={`status-pill status-pill--${entry.status}`}>{entry.status}</span>
              </div>
            ))}
          </div>
        ) : (
          <div className="empty-state">
            <h3>Nadie está esperando</h3>
            <p>Cuando un día se complete, ofrecé un lugar desde esta lista.</p>
          </div>
        )}
      </section>
      <aside className="operation-panel__side">
        <header className="section-heading">
          <div>
            <p className="eyebrow">Carga manual</p>
            <h2>Agregar solicitud</h2>
          </div>
        </header>
        <form className="stack-form" onSubmit={(event) => void submit(event)}>
          <label className="field">
            <span>Cliente</span>
            <input value={name} onChange={(event) => setName(event.target.value)} required />
          </label>
          <label className="field">
            <span>Email</span>
            <input type="email" value={email} onChange={(event) => setEmail(event.target.value)} />
          </label>
          <label className="field">
            <span>Servicio</span>
            <select value={serviceId} onChange={(event) => setServiceId(event.target.value)} required>
              {services.map((service) => (
                <option key={service.id} value={service.id}>
                  {service.name}
                </option>
              ))}
            </select>
          </label>
          <label className="field">
            <span>Día preferido</span>
            <input type="date" value={day} onChange={(event) => setDay(event.target.value)} required />
          </label>
          <button type="submit" className="button button--primary" disabled={pending}>
            {pending ? "Agregando…" : "Agregar a espera"}
          </button>
        </form>
      </aside>
    </div>
  );
}
