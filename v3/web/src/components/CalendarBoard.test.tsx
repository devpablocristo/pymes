import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { DateTime } from "luxon";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Booking } from "../domain/scheduling";

const revert = vi.fn();

vi.mock("@devpablocristo/platform-calendar-board/next", () => ({
  CalendarSurface: (props: {
    onEventDrop: (arg: unknown) => void;
    onEventResize: (arg: unknown) => void;
    editable: boolean;
    selectable: boolean;
    eventDurationEditable: boolean;
    events: Array<{ editable?: boolean }>;
  }) => (
    <>
      <output
        data-testid="calendar-capabilities"
        data-editable={String(props.editable)}
        data-selectable={String(props.selectable)}
        data-duration-editable={String(props.eventDurationEditable)}
        data-event-editable={String(props.events[0]?.editable)}
      />
      <button
        type="button"
        onClick={() =>
          props.onEventDrop({
            event: {
              start: new Date("2026-08-04T13:00:00.000Z"),
              end: new Date("2026-08-04T14:00:00.000Z"),
              extendedProps: { booking },
            },
            revert,
          })
        }
      >
        Simular arrastre
      </button>
      <button
        type="button"
        onClick={() =>
          props.onEventResize({
            event: {
              start: new Date("2026-08-04T13:00:00.000Z"),
              end: new Date("2026-08-04T15:00:00.000Z"),
              extendedProps: { booking },
            },
            revert,
          })
        }
      >
        Simular resize
      </button>
    </>
  ),
}));

const booking: Booking = {
  id: "88888888-8888-4888-8888-888888888888",
  branch_id: "11111111-1111-4111-8111-111111111111",
  service_id: "33333333-3333-4333-8333-333333333333",
  party_id: "party-one",
  status: "confirmed",
  participants: 1,
  start_at: "2026-08-04T12:00:00.000Z",
  end_at: "2026-08-04T13:00:00.000Z",
  version: 1,
  service_name: "Consulta",
  price: "100.00",
  currency: "ARS",
  duration_minutes: 60,
  timezone: "America/Argentina/Buenos_Aires",
  allocations: [],
};

describe("CalendarBoard", () => {
  beforeEach(() => revert.mockReset());

  it.each(["Simular arrastre", "Simular resize"])(
    "revierte visualmente %s cuando PostgreSQL rechaza el cambio",
    async (label) => {
      const onMutationError = vi.fn();
      const onReschedule = vi.fn().mockRejectedValue(new Error("RESOURCE_CONFLICT"));
      const { CalendarBoard } = await import("./CalendarBoard");
      render(
        <CalendarBoard
          bookings={[booking]}
          blocks={[]}
          loaded
          timezone="America/Argentina/Buenos_Aires"
          canOperate
          onRangeChange={vi.fn()}
          onCreateAt={vi.fn()}
          onSelectBooking={vi.fn()}
          onReschedule={onReschedule}
          onMutationError={onMutationError}
        />,
      );
      fireEvent.click(screen.getByRole("button", { name: label }));
      await waitFor(() => expect(revert).toHaveBeenCalledOnce());
      expect(onMutationError).toHaveBeenCalledOnce();
      expect(onReschedule).toHaveBeenCalledWith(
        booking,
        DateTime.fromISO("2026-08-04T13:00:00.000Z").toJSDate().toISOString(),
        DateTime.fromISO(label.includes("resize") ? "2026-08-04T15:00:00.000Z" : "2026-08-04T14:00:00.000Z")
          .toJSDate()
          .toISOString(),
      );
    },
  );

  it("deshabilita selección y mutaciones para sesiones de sólo lectura", async () => {
    const { CalendarBoard } = await import("./CalendarBoard");
    render(
      <CalendarBoard
        bookings={[booking]}
        blocks={[]}
        loaded
        timezone="America/Argentina/Buenos_Aires"
        canOperate={false}
        onRangeChange={vi.fn()}
        onCreateAt={vi.fn()}
        onSelectBooking={vi.fn()}
        onReschedule={vi.fn()}
        onMutationError={vi.fn()}
      />,
    );

    const capabilities = screen.getByTestId("calendar-capabilities");
    expect(capabilities).toHaveAttribute("data-editable", "false");
    expect(capabilities).toHaveAttribute("data-selectable", "false");
    expect(capabilities).toHaveAttribute("data-duration-editable", "false");
    expect(capabilities).toHaveAttribute("data-event-editable", "false");
  });
});
