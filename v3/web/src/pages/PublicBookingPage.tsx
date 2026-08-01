import { useQuery } from "@tanstack/react-query";
import { formatSchedulingDateOnly } from "@devpablocristo/platform-scheduling/next";
import { DateTime } from "luxon";
import { type FormEvent, useEffect, useMemo, useState } from "react";
import { useSchedulingGateway } from "../api/GatewayContext";
import { errorMessage } from "../api/errors";
import type { PublicBooking, PublicCatalog, Slot } from "../domain/scheduling";

type Step = 1 | 2 | 3 | 4;

function titleFromSlug(slug: string): string {
  return slug
    .split("-")
    .map((word) => `${word.charAt(0).toUpperCase()}${word.slice(1)}`)
    .join(" ");
}

export function PublicBookingPage({
  defaultSlug,
  organizationSlug: routeSlug,
}: {
  defaultSlug: string;
  organizationSlug: string;
}) {
  const organizationSlug = routeSlug || defaultSlug;
  const gateway = useSchedulingGateway();
  const [step, setStep] = useState<Step>(1);
  const [branchId, setBranchId] = useState("");
  const [serviceId, setServiceId] = useState("");
  const [professionalId, setProfessionalId] = useState("");
  const [day, setDay] = useState(DateTime.now().plus({ days: 1 }).toISODate() ?? "");
  const [participants, setParticipants] = useState(1);
  const [selectedSlot, setSelectedSlot] = useState<Slot | null>(null);
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [notes, setNotes] = useState("");
  const [booking, setBooking] = useState<PublicBooking | null>(null);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [waitlisted, setWaitlisted] = useState(false);
  const steps: ReadonlyArray<{ label: string; number: Step }> = [
    { label: "Servicio", number: 1 },
    { label: "Horario", number: 2 },
    { label: "Tus datos", number: 3 },
    { label: "Listo", number: 4 },
  ];

  const catalogQuery = useQuery({
    queryKey: ["public-scheduling", organizationSlug, "catalog"],
    queryFn: () => gateway.getPublicCatalog(organizationSlug),
  });
  const catalog: PublicCatalog | undefined = catalogQuery.data;

  useEffect(() => {
    if (!branchId && catalog?.branches[0]) {
      setBranchId(catalog.branches[0].id);
    }
  }, [branchId, catalog]);

  useEffect(() => {
    setProfessionalId("");
    setSelectedSlot(null);
  }, [branchId, serviceId]);

  const branch = catalog?.branches.find((item) => item.id === branchId);
  const service = catalog?.services.find((item) => item.id === serviceId);
  const timezone = branch?.timezone ?? "America/Argentina/Buenos_Aires";
  const professionals = useMemo(
    () =>
      catalog?.resources.filter((resource) => resource.branch_id === branchId && resource.kind === "professional") ??
      [],
    [branchId, catalog?.resources],
  );
  const from = DateTime.fromISO(day, { zone: timezone }).startOf("day").toUTC().toISO()!;
  const until = DateTime.fromISO(day, { zone: timezone }).endOf("day").toUTC().toISO()!;

  const availabilityQuery = useQuery({
    queryKey: [
      "public-scheduling",
      organizationSlug,
      "availability",
      branchId,
      serviceId,
      professionalId,
      day,
      participants,
    ],
    queryFn: () =>
      gateway.calculatePublicAvailability(organizationSlug, {
        branch_id: branchId,
        service_id: serviceId,
        from,
        until,
        participants,
        ...(professionalId
          ? { allocations: [{ resource_id: professionalId, mode: "exclusive" as const, units: 1 }] }
          : {}),
      }),
    enabled: Boolean(branchId && serviceId && day),
  });

  async function submitBooking(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedSlot || !service) return;
    setPending(true);
    setError(null);
    try {
      const customer = { name, ...(email ? { email } : {}), ...(phone ? { phone } : {}) };
      const created = await gateway.createPublicBooking(organizationSlug, {
        branch_id: branchId,
        service_id: service.id,
        customer,
        start_at: selectedSlot.start_at,
        participants,
        status: service.confirmation_required ? "pending_confirmation" : "confirmed",
        allocations: selectedSlot.allocations,
        ...(notes ? { notes } : {}),
      });
      setBooking(created[0] ?? null);
      setStep(4);
    } catch (caught) {
      setSelectedSlot(null);
      await availabilityQuery.refetch();
      setError(errorMessage(caught));
    } finally {
      setPending(false);
    }
  }

  async function joinWaitlist() {
    if (!service || !name) {
      setError("Ingresá tu nombre antes de sumarte a la lista de espera.");
      setStep(3);
      return;
    }
    setPending(true);
    try {
      await gateway.createPublicWaitlistEntry(organizationSlug, {
        branch_id: branchId,
        service_id: service.id,
        customer: { name, ...(email ? { email } : {}), ...(phone ? { phone } : {}) },
        preferred_from: from,
        preferred_until: until,
        participants,
      });
      setWaitlisted(true);
      setStep(4);
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setPending(false);
    }
  }

  if (catalogQuery.isLoading) {
    return (
      <main className="public-loading" id="main-content" aria-live="polite">
        <span className="public-brand-mark">P</span>
        <p>Cargando horarios disponibles…</p>
      </main>
    );
  }

  if (catalogQuery.isError || !catalog) {
    return (
      <main className="public-error" id="main-content">
        <p className="eyebrow">Agenda no encontrada</p>
        <h1>No pudimos abrir esta página de reservas</h1>
        <p>{errorMessage(catalogQuery.error)}</p>
      </main>
    );
  }

  return (
    <main className="public-booking" id="main-content">
      <header className="public-booking__header">
        <div className="public-brand">
          <span className="public-brand-mark">P</span>
          <span>Reservas seguras con Pymes</span>
        </div>
        <div>
          <p className="eyebrow">Agenda online</p>
          <h1>{titleFromSlug(organizationSlug)}</h1>
          <p>Elegí un horario. La disponibilidad se confirma al reservar.</p>
        </div>
      </header>

      <div className="public-booking__body">
        <ol className="booking-steps" aria-label="Pasos de la reserva">
          {steps.map(({ label, number }) => (
            <li key={number} className={step === number ? "active" : step > number ? "complete" : ""}>
              <span>{step > number ? "✓" : number}</span>
              {label}
            </li>
          ))}
        </ol>

        {error ? (
          <div className="inline-alert" role="alert">
            {error}
          </div>
        ) : null}

        {step === 1 ? (
          <section className="booking-step-panel" aria-labelledby="step-service-title">
            <header>
              <p className="eyebrow">Paso 1</p>
              <h2 id="step-service-title">¿Qué necesitás reservar?</h2>
            </header>
            <div className="public-form-grid">
              <label className="field">
                <span>Sucursal</span>
                <select value={branchId} onChange={(event) => setBranchId(event.target.value)}>
                  {catalog.branches.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name}
                    </option>
                  ))}
                </select>
                {branch?.address ? <small>{branch.address}</small> : null}
              </label>
              <fieldset className="service-cards">
                <legend>Servicio</legend>
                {catalog.services.map((item) => (
                  <label key={item.id} className={serviceId === item.id ? "selected" : ""}>
                    <input
                      type="radio"
                      name="service"
                      value={item.id}
                      checked={serviceId === item.id}
                      onChange={() => setServiceId(item.id)}
                    />
                    <span>
                      <strong>{item.name}</strong>
                      <small>
                        {item.duration_minutes} min · {item.fulfillment_mode === "virtual" ? "Virtual" : "Presencial"}
                      </small>
                    </span>
                    <b>
                      {item.currency} {item.price}
                    </b>
                  </label>
                ))}
              </fieldset>
              <label className="field">
                <span>Profesional</span>
                <select value={professionalId} onChange={(event) => setProfessionalId(event.target.value)}>
                  <option value="">Primero disponible</option>
                  {professionals.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name}
                    </option>
                  ))}
                </select>
              </label>
              {service?.allow_group ? (
                <label className="field">
                  <span>Participantes</span>
                  <input
                    type="number"
                    min={1}
                    max={service.max_participants}
                    value={participants}
                    onChange={(event) => setParticipants(Number(event.target.value))}
                  />
                </label>
              ) : null}
            </div>
            <footer>
              <button
                type="button"
                className="button button--primary"
                disabled={!branchId || !serviceId}
                onClick={() => setStep(2)}
              >
                Ver horarios
              </button>
            </footer>
          </section>
        ) : null}

        {step === 2 ? (
          <section className="booking-step-panel" aria-labelledby="step-slot-title">
            <header>
              <p className="eyebrow">Paso 2 · {branch?.name}</p>
              <h2 id="step-slot-title">Elegí el horario que te conviene</h2>
            </header>
            <label className="public-date-picker">
              <span>Fecha</span>
              <input
                type="date"
                min={DateTime.now().setZone(timezone).toISODate() ?? undefined}
                value={day}
                onChange={(event) => {
                  setDay(event.target.value);
                  setSelectedSlot(null);
                }}
              />
            </label>
            {availabilityQuery.isLoading ? <p aria-live="polite">Buscando horarios…</p> : null}
            {availabilityQuery.data?.length ? (
              <fieldset className="slot-grid">
                <legend>
                  {formatSchedulingDateOnly(from, "es-AR")} · hora local {timezone}
                </legend>
                {availabilityQuery.data.map((slot) => {
                  const start = DateTime.fromISO(slot.start_at).setZone(timezone);
                  return (
                    <label key={`${slot.start_at}:${slot.allocations.map((item) => item.resource_id).join("-")}`}>
                      <input
                        type="radio"
                        name="slot"
                        checked={selectedSlot?.start_at === slot.start_at}
                        onChange={() => setSelectedSlot(slot)}
                      />
                      <span>{start.toFormat("HH:mm")}</span>
                      <small>{slot.remaining} disponibles</small>
                    </label>
                  );
                })}
              </fieldset>
            ) : !availabilityQuery.isLoading ? (
              <div className="empty-state">
                <h3>No quedan horarios ese día</h3>
                <p>Probá otra fecha o sumate a la lista de espera.</p>
                {service?.allow_waitlist ? (
                  <button type="button" className="button button--secondary" onClick={() => setStep(3)}>
                    Ir a lista de espera
                  </button>
                ) : null}
              </div>
            ) : null}
            <footer className="split-actions">
              <button type="button" className="button button--quiet" onClick={() => setStep(1)}>
                Volver
              </button>
              <button
                type="button"
                className="button button--primary"
                disabled={!selectedSlot}
                onClick={() => setStep(3)}
              >
                Continuar
              </button>
            </footer>
          </section>
        ) : null}

        {step === 3 ? (
          <section className="booking-step-panel" aria-labelledby="step-contact-title">
            <header>
              <p className="eyebrow">Paso 3</p>
              <h2 id="step-contact-title">{selectedSlot ? "¿A nombre de quién reservamos?" : "Sumate a la espera"}</h2>
            </header>
            <form className="public-contact-form" onSubmit={(event) => void submitBooking(event)}>
              <label className="field">
                <span>Nombre y apellido</span>
                <input value={name} onChange={(event) => setName(event.target.value)} autoComplete="name" required />
              </label>
              <label className="field">
                <span>Email</span>
                <input
                  type="email"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  autoComplete="email"
                />
              </label>
              <label className="field">
                <span>WhatsApp</span>
                <input
                  type="tel"
                  value={phone}
                  onChange={(event) => setPhone(event.target.value)}
                  autoComplete="tel"
                  required
                />
              </label>
              <label className="field field--wide">
                <span>Comentario para el equipo (opcional)</span>
                <textarea value={notes} onChange={(event) => setNotes(event.target.value)} maxLength={500} />
              </label>
              <div className="booking-summary">
                <strong>{service?.name}</strong>
                <span>
                  {selectedSlot
                    ? `${DateTime.fromISO(selectedSlot.start_at).setZone(timezone).toFormat("cccc d 'de' LLLL · HH:mm", {
                        locale: "es",
                      })}`
                    : `Preferencia: ${formatSchedulingDateOnly(from, "es-AR")}`}
                </span>
              </div>
              <footer className="split-actions">
                <button type="button" className="button button--quiet" onClick={() => setStep(2)}>
                  Volver
                </button>
                {selectedSlot ? (
                  <button type="submit" className="button button--primary" disabled={pending}>
                    {pending ? "Confirmando…" : "Confirmar reserva"}
                  </button>
                ) : (
                  <button
                    type="button"
                    className="button button--primary"
                    disabled={pending}
                    onClick={() => void joinWaitlist()}
                  >
                    {pending ? "Agregando…" : "Sumarme a la espera"}
                  </button>
                )}
              </footer>
            </form>
          </section>
        ) : null}

        {step === 4 ? (
          <section className="booking-complete" aria-live="polite">
            <span className="booking-complete__mark">✓</span>
            <p className="eyebrow">{waitlisted ? "Lista de espera" : "Reserva recibida"}</p>
            <h2>{waitlisted ? "Te avisaremos si se libera un lugar" : "Tu horario quedó reservado"}</h2>
            {booking ? (
              <p>
                {DateTime.fromISO(booking.start_at).setZone(timezone).toFormat("cccc d 'de' LLLL · HH:mm", {
                  locale: "es",
                })}
                . Estado: {booking.status === "pending_confirmation" ? "a confirmar" : "confirmado"}.
              </p>
            ) : (
              <p>Guardamos tu preferencia para {formatSchedulingDateOnly(from, "es-AR")}.</p>
            )}
            <p className="muted">La empresa enviará los enlaces de confirmación, cancelación o reprogramación.</p>
          </section>
        ) : null}
      </div>
    </main>
  );
}
