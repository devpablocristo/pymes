import { DateTime } from "luxon";
import { type FormEvent, useEffect, useState } from "react";
import type { AvailabilityBlockInput, Branch, Resource } from "../domain/scheduling";
import { FormDialog } from "./Dialog";

type BlockDialogProps = {
  open: boolean;
  branch: Branch | null;
  resources: Resource[];
  initialStart?: string;
  initialEnd?: string;
  pending: boolean;
  onClose: () => void;
  onSave: (input: AvailabilityBlockInput) => Promise<void>;
};

function localValue(iso: string | undefined, timezone: string): string {
  return DateTime.fromISO(iso ?? DateTime.now().toISO()!, { setZone: true })
    .setZone(timezone)
    .toFormat("yyyy-MM-dd'T'HH:mm");
}

export function BlockDialog({
  open,
  branch,
  resources,
  initialStart,
  initialEnd,
  pending,
  onClose,
  onSave,
}: BlockDialogProps) {
  const timezone = branch?.timezone ?? "America/Argentina/Buenos_Aires";
  const [kind, setKind] = useState<AvailabilityBlockInput["kind"]>("manual");
  const [resourceId, setResourceId] = useState("");
  const [startAt, setStartAt] = useState("");
  const [endAt, setEndAt] = useState("");
  const [reason, setReason] = useState("");

  useEffect(() => {
    if (!open) return;
    setKind("manual");
    setResourceId("");
    const start = initialStart ?? DateTime.now().plus({ hours: 1 }).startOf("hour").toISO()!;
    const end = initialEnd ?? DateTime.fromISO(start).plus({ hours: 1 }).toISO()!;
    setStartAt(localValue(start, timezone));
    setEndAt(localValue(end, timezone));
    setReason("");
  }, [initialEnd, initialStart, open, timezone]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!branch) return;
    const input: AvailabilityBlockInput = {
      branch_id: branch.id,
      kind,
      start_at: DateTime.fromFormat(startAt, "yyyy-MM-dd'T'HH:mm", { zone: timezone }).toUTC().toISO()!,
      end_at: DateTime.fromFormat(endAt, "yyyy-MM-dd'T'HH:mm", { zone: timezone }).toUTC().toISO()!,
    };
    if (resourceId) input.resource_id = resourceId;
    if (reason) input.reason = reason;
    await onSave(input);
  }

  return (
    <FormDialog
      open={open}
      formId="block-form"
      title="Bloquear disponibilidad"
      eyebrow={branch?.name ?? "Agenda"}
      submitLabel="Crear bloqueo"
      pending={pending}
      onClose={onClose}
      onSubmit={(event) => void submit(event)}
    >
      <div className="form-grid">
        <label className="field">
          <span>Motivo operativo</span>
          <select value={kind} onChange={(event) => setKind(event.target.value as AvailabilityBlockInput["kind"])}>
            <option value="manual">Bloqueo manual</option>
            <option value="holiday">Feriado</option>
            <option value="vacation">Vacaciones</option>
            <option value="absence">Ausencia</option>
            <option value="maintenance">Mantenimiento</option>
            <option value="availability">Disponibilidad excepcional</option>
          </select>
        </label>
        <label className="field">
          <span>Alcance</span>
          <select value={resourceId} onChange={(event) => setResourceId(event.target.value)}>
            <option value="">Toda la sucursal</option>
            {resources.map((resource) => (
              <option key={resource.id} value={resource.id}>
                {resource.name}
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span>Desde</span>
          <input
            type="datetime-local"
            value={startAt}
            onChange={(event) => setStartAt(event.target.value)}
            required
          />
        </label>
        <label className="field">
          <span>Hasta</span>
          <input type="datetime-local" value={endAt} onChange={(event) => setEndAt(event.target.value)} required />
        </label>
      </div>
      <label className="field">
        <span>Detalle visible para el equipo</span>
        <input value={reason} onChange={(event) => setReason(event.target.value)} maxLength={250} />
      </label>
    </FormDialog>
  );
}
