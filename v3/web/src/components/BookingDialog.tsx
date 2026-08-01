import { DateTime } from "luxon";
import { type FormEvent, useEffect, useMemo, useState } from "react";
import type {
  Allocation,
  Booking,
  BookingInput,
  Branch,
  Resource,
  Service,
} from "../domain/scheduling";
import { resourceLabel } from "../domain/scheduling";
import { FormDialog } from "./Dialog";

type NewBookingDraft = {
  startAt: string;
  endAt: string;
};

type BookingDialogProps = {
  open: boolean;
  draft: NewBookingDraft | null;
  booking: Booking | null;
  branchId: string;
  timezone: string;
  branches: Branch[];
  services: Service[];
  resources: Resource[];
  preferredServiceId?: string;
  preferredResourceId?: string;
  pending: boolean;
  onClose: () => void;
  onCreate: (input: BookingInput) => Promise<void>;
  onReschedule: (
    booking: Booking,
    startAt: string,
    endAt: string,
    durationMinutes: number,
    allocations: Allocation[],
  ) => Promise<void>;
};

function toLocalInput(value: string, zone: string): string {
  return DateTime.fromISO(value, { setZone: true }).setZone(zone).toFormat("yyyy-MM-dd'T'HH:mm");
}

function fromLocalInput(value: string, zone: string): string {
  return DateTime.fromFormat(value, "yyyy-MM-dd'T'HH:mm", { zone }).toUTC().toISO() ?? value;
}

export function BookingDialog({
  open,
  draft,
  booking,
  branchId,
  timezone,
  branches,
  services,
  resources,
  preferredServiceId,
  preferredResourceId,
  pending,
  onClose,
  onCreate,
  onReschedule,
}: BookingDialogProps) {
  const [selectedBranchId, setSelectedBranchId] = useState(branchId);
  const [serviceId, setServiceId] = useState(preferredServiceId ?? "");
  const [startAt, setStartAt] = useState("");
  const [durationMinutes, setDurationMinutes] = useState(30);
  const [customerName, setCustomerName] = useState("");
  const [customerEmail, setCustomerEmail] = useState("");
  const [customerPhone, setCustomerPhone] = useState("");
  const [participants, setParticipants] = useState(1);
  const [notes, setNotes] = useState("");
  const [allocationIds, setAllocationIds] = useState<string[]>([]);

  const selectedService = services.find((item) => item.id === serviceId);
  const branchResources = useMemo(
    () => resources.filter((resource) => resource.branch_id === selectedBranchId),
    [resources, selectedBranchId],
  );

  useEffect(() => {
    if (!open) return;
    const effectiveStart = booking?.start_at ?? draft?.startAt ?? DateTime.now().toISO()!;
    const effectiveServiceId = booking?.service_id ?? preferredServiceId ?? services[0]?.id ?? "";
    const effectiveService = services.find((item) => item.id === effectiveServiceId);
    setSelectedBranchId(booking?.branch_id ?? branchId);
    setServiceId(effectiveServiceId);
    setStartAt(toLocalInput(effectiveStart, timezone));
    setDurationMinutes(booking?.duration_minutes ?? effectiveService?.duration_minutes ?? 30);
    setCustomerName(booking ? `Cliente ${booking.party_id.slice(-6)}` : "");
    setCustomerEmail("");
    setCustomerPhone("");
    setParticipants(booking?.participants ?? 1);
    setNotes("");
    setAllocationIds(
      booking?.allocations.map((item) => item.resource_id) ??
        (preferredResourceId ? [preferredResourceId] : []),
    );
  }, [booking, branchId, draft?.startAt, open, preferredResourceId, preferredServiceId, services, timezone]);

  useEffect(() => {
    if (!booking && selectedService) {
      setDurationMinutes(selectedService.duration_minutes);
    }
  }, [booking, selectedService]);

  function toggleResource(resourceId: string) {
    setAllocationIds((current) =>
      current.includes(resourceId) ? current.filter((id) => id !== resourceId) : [...current, resourceId],
    );
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const start = fromLocalInput(startAt, timezone);
    const end =
      DateTime.fromISO(start, { setZone: true }).plus({ minutes: durationMinutes }).toUTC().toISO() ?? start;
    const allocations: Allocation[] = allocationIds.map((resourceId) => ({
      resource_id: resourceId,
      mode: "exclusive",
      units: 1,
    }));
    if (booking) {
      await onReschedule(booking, start, end, durationMinutes, allocations);
      return;
    }
    if (!selectedService) return;
    const customer: BookingInput["customer"] = { name: customerName };
    if (customerEmail) customer.email = customerEmail;
    if (customerPhone) customer.phone = customerPhone;
    const input: BookingInput = {
      branch_id: selectedBranchId,
      service_id: selectedService.id,
      customer,
      start_at: start,
      participants,
      status: selectedService.confirmation_required ? "pending_confirmation" : "confirmed",
      allocations,
    };
    if (notes) input.notes = notes;
    await onCreate(input);
  }

  const selectedEnd = startAt
    ? DateTime.fromFormat(startAt, "yyyy-MM-dd'T'HH:mm", { zone: timezone }).plus({ minutes: durationMinutes })
    : null;

  return (
    <FormDialog
      open={open}
      formId="booking-form"
      title={booking ? "Reprogramar turno" : "Nuevo turno"}
      eyebrow={booking ? "Nueva versión del turno" : "Reserva interna"}
      submitLabel={booking ? "Guardar reprogramación" : "Crear turno"}
      pending={pending}
      onClose={onClose}
      onSubmit={(event) => void submit(event)}
      size="large"
    >
      <div className="form-grid">
        <label className="field">
          <span>Sucursal</span>
          <select
            value={selectedBranchId}
            onChange={(event) => setSelectedBranchId(event.target.value)}
            disabled={Boolean(booking)}
            required
          >
            {branches.map((branch) => (
              <option key={branch.id} value={branch.id}>
                {branch.name}
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span>Servicio</span>
          <select
            value={serviceId}
            onChange={(event) => setServiceId(event.target.value)}
            disabled={Boolean(booking)}
            required
          >
            <option value="">Elegí un servicio</option>
            {services.map((service) => (
              <option key={service.id} value={service.id}>
                {service.name} · {service.duration_minutes} min
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span>Comienza</span>
          <input
            type="datetime-local"
            value={startAt}
            onChange={(event) => setStartAt(event.target.value)}
            required
          />
        </label>
        <label className="field">
          <span>Duración</span>
          <div className="field__suffix">
            <input
              type="number"
              min={1}
              max={1440}
              value={durationMinutes}
              onChange={(event) => setDurationMinutes(Number(event.target.value))}
              required
            />
            <span>min</span>
          </div>
          {selectedEnd ? <small>Finaliza {selectedEnd.toFormat("HH:mm")}</small> : null}
        </label>
      </div>

      {!booking ? (
        <>
          <div className="form-grid form-grid--three">
            <label className="field">
              <span>Cliente</span>
              <input
                value={customerName}
                onChange={(event) => setCustomerName(event.target.value)}
                autoComplete="name"
                required
              />
            </label>
            <label className="field">
              <span>Email</span>
              <input
                type="email"
                value={customerEmail}
                onChange={(event) => setCustomerEmail(event.target.value)}
                autoComplete="email"
              />
            </label>
            <label className="field">
              <span>WhatsApp</span>
              <input
                type="tel"
                value={customerPhone}
                onChange={(event) => setCustomerPhone(event.target.value)}
                autoComplete="tel"
              />
            </label>
          </div>
          <div className="form-grid">
            <label className="field">
              <span>Participantes</span>
              <input
                type="number"
                min={1}
                max={selectedService?.max_participants ?? 1}
                value={participants}
                onChange={(event) => setParticipants(Number(event.target.value))}
              />
            </label>
            <label className="field">
              <span>Nota interna</span>
              <input value={notes} onChange={(event) => setNotes(event.target.value)} maxLength={500} />
            </label>
          </div>
        </>
      ) : null}

      <fieldset className="resource-picker">
        <legend>Recursos asignados</legend>
        <p>La API revalida todos juntos antes de confirmar.</p>
        <div className="resource-picker__grid">
          {branchResources.map((resource) => (
            <label key={resource.id} className="resource-choice">
              <input
                type="checkbox"
                checked={allocationIds.includes(resource.id)}
                onChange={() => toggleResource(resource.id)}
              />
              <span>
                <strong>{resource.name}</strong>
                <small>{resourceLabel(resource).split(" · ")[0]}</small>
              </span>
            </label>
          ))}
        </div>
      </fieldset>
    </FormDialog>
  );
}

export type { NewBookingDraft };
