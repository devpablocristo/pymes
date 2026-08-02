import { type FormEvent, useEffect, useState } from "react";
import type { Booking } from "../domain/scheduling";
import { FormDialog } from "./Dialog";

export function CancellationDialog({
  booking,
  pending,
  onClose,
  onConfirm,
}: {
  booking: Booking | null;
  pending: boolean;
  onClose: () => void;
  onConfirm: (reason: string) => Promise<void>;
}) {
  const [reason, setReason] = useState("");

  useEffect(() => {
    if (booking) setReason("");
  }, [booking]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await onConfirm(reason.trim());
  }

  return (
    <FormDialog
      open={Boolean(booking)}
      formId="cancel-booking-form"
      title="Cancelar turno"
      eyebrow={booking?.service_name ?? "Agenda"}
      submitLabel="Confirmar cancelación"
      pending={pending}
      onClose={onClose}
      onSubmit={(event) => void submit(event)}
    >
      <p className="dialog-explainer">
        El horario volverá a quedar disponible y la cancelación quedará en el historial.
      </p>
      <label className="field">
        <span>Motivo de cancelación</span>
        <textarea
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          minLength={3}
          maxLength={500}
          required
          autoFocus
        />
      </label>
    </FormDialog>
  );
}
