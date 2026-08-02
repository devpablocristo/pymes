import { type FormEvent, useEffect, useState } from "react";
import type { Booking, BookingUpdateInput } from "../domain/scheduling";
import { FormDialog } from "./Dialog";

type BookingEditDialogProps = {
  booking: Booking | null;
  maxParticipants: number;
  pending: boolean;
  onClose: () => void;
  onSave: (booking: Booking, input: BookingUpdateInput) => Promise<void>;
};

export function BookingEditDialog({
  booking,
  maxParticipants,
  pending,
  onClose,
  onSave,
}: BookingEditDialogProps) {
  const [customerEmail, setCustomerEmail] = useState("");
  const [customerPhone, setCustomerPhone] = useState("");
  const [participants, setParticipants] = useState(1);
  const [notes, setNotes] = useState("");
  const [substateCode, setSubstateCode] = useState("");

  useEffect(() => {
    if (!booking) return;
    setCustomerEmail(booking.customer_email ?? "");
    setCustomerPhone(booking.customer_phone ?? "");
    setParticipants(booking.participants);
    setNotes(booking.notes ?? "");
    setSubstateCode(booking.substate_code ?? "");
  }, [booking]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!booking) return;
    const input: BookingUpdateInput = { expected_version: booking.version };
    if (
      customerEmail.trim() !== (booking.customer_email ?? "") ||
      customerPhone.trim() !== (booking.customer_phone ?? "")
    ) {
      input.customer = {
        party_id: booking.party_id,
        name: booking.customer_name || `Cliente ${booking.party_id.slice(-6)}`,
      };
      if (customerEmail.trim()) input.customer.email = customerEmail.trim();
      if (customerPhone.trim()) input.customer.phone = customerPhone.trim();
    }
    if (participants !== booking.participants) input.participants = participants;
    if (notes.trim() !== (booking.notes ?? "")) input.notes = notes.trim();
    if (substateCode.trim() !== (booking.substate_code ?? "")) {
      input.substate_code = substateCode.trim();
    }
    if (Object.keys(input).length === 1) {
      onClose();
      return;
    }
    await onSave(booking, input);
  }

  return (
    <FormDialog
      open={Boolean(booking)}
      formId="booking-edit-form"
      title="Editar turno"
      eyebrow="Datos operativos"
      submitLabel="Guardar cambios"
      pending={pending}
      onClose={onClose}
      onSubmit={(event) => void submit(event).catch(() => undefined)}
      size="large"
    >
      <div className="form-grid form-grid--three">
        <label className="field">
          <span>Cliente</span>
          <input value={booking?.customer_name ?? ""} readOnly aria-readonly="true" />
          <small>La identidad se administra desde Clientes.</small>
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
            max={Math.max(1, maxParticipants)}
            value={participants}
            onChange={(event) => setParticipants(Number(event.target.value))}
            required
          />
          <small>El cupo y los recursos se revalidan al guardar.</small>
        </label>
        <label className="field">
          <span>Subestado operativo</span>
          <input
            value={substateCode}
            onChange={(event) => setSubstateCode(event.target.value)}
            maxLength={40}
            pattern="[a-z][a-z0-9_-]{0,39}"
            placeholder="Sin subestado"
          />
          <small>Debe estar habilitado para el estado actual.</small>
        </label>
      </div>
      <label className="field">
        <span>Nota interna</span>
        <textarea
          value={notes}
          onChange={(event) => setNotes(event.target.value)}
          maxLength={2000}
          rows={4}
        />
      </label>
      <p className="muted">
        Para cambiar fecha, duración o recursos usá Reprogramar. El estado se modifica con sus acciones.
      </p>
    </FormDialog>
  );
}
