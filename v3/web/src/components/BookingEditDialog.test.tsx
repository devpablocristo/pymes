import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, it, vi } from "vitest";
import type { Booking } from "../domain/scheduling";
import { BookingEditDialog } from "./BookingEditDialog";

beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function showModal() {
    this.open = true;
  };
  HTMLDialogElement.prototype.close = function close() {
    this.open = false;
  };
});

const booking: Booking = {
  id: "88888888-8888-4888-8888-888888888888",
  branch_id: "11111111-1111-4111-8111-111111111111",
  service_id: "33333333-3333-4333-8333-333333333333",
  party_id: "party-one",
  status: "confirmed",
  participants: 1,
  start_at: "2026-08-04T12:00:00.000Z",
  end_at: "2026-08-04T13:00:00.000Z",
  version: 7,
  service_name: "Consulta",
  price: "100.00",
  currency: "ARS",
  duration_minutes: 60,
  timezone: "America/Argentina/Buenos_Aires",
  customer_name: "Ada Lovelace",
  customer_email: "anterior@example.com",
  customer_phone: "+541111111111",
  notes: "Nota anterior",
  allocations: [],
};

describe("BookingEditDialog", () => {
  it("envía sólo campos operativos con la versión optimista", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <BookingEditDialog
        booking={booking}
        maxParticipants={4}
        pending={false}
        onClose={vi.fn()}
        onSave={onSave}
      />,
    );
    const customer = screen.getByLabelText(/^Cliente/);
    expect(customer).toHaveAttribute("readonly");
    await user.clear(screen.getByLabelText("Email"));
    await user.type(screen.getByLabelText("Email"), "ada@example.com");
    await user.clear(screen.getByLabelText(/^Participantes/));
    await user.type(screen.getByLabelText(/^Participantes/), "2");
    await user.clear(screen.getByLabelText(/^Subestado operativo/));
    await user.type(screen.getByLabelText(/^Subestado operativo/), "first_visit");
    await user.clear(screen.getByLabelText("Nota interna"));
    await user.type(screen.getByLabelText("Nota interna"), "Acceso por recepción");
    await user.click(screen.getByRole("button", { name: "Guardar cambios" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledOnce());
    const [, input] = onSave.mock.calls[0]!;
    expect(input).toEqual({
      expected_version: 7,
      customer: {
        party_id: "party-one",
        name: "Ada Lovelace",
        email: "ada@example.com",
        phone: "+541111111111",
      },
      participants: 2,
      notes: "Acceso por recepción",
      substate_code: "first_visit",
    });
    expect(input).not.toHaveProperty("start_at");
    expect(input).not.toHaveProperty("service_id");
    expect(input).not.toHaveProperty("status");
    expect(input).not.toHaveProperty("price");
  });

  it("cierra sin mutar cuando no hay cambios", async () => {
    const onClose = vi.fn();
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <BookingEditDialog
        booking={booking}
        maxParticipants={4}
        pending={false}
        onClose={onClose}
        onSave={onSave}
      />,
    );
    fireEvent.submit(document.querySelector("#booking-edit-form")!);
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
    expect(onSave).not.toHaveBeenCalled();
  });

  it("mantiene el diálogo abierto cuando el backend rechaza la versión", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onSave = vi.fn().mockRejectedValue(new Error("BOOKING_VERSION_CONFLICT"));
    render(
      <BookingEditDialog
        booking={booking}
        maxParticipants={4}
        pending={false}
        onClose={onClose}
        onSave={onSave}
      />,
    );
    await user.clear(screen.getByLabelText("Nota interna"));
    await user.type(screen.getByLabelText("Nota interna"), "Cambio concurrente");
    await user.click(screen.getByRole("button", { name: "Guardar cambios" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledOnce());
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: "Editar turno" })).toBeVisible();
  });
});
