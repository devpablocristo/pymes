import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, it, vi } from "vitest";
import { InMemorySchedulingGateway } from "../api/fakeSchedulingGateway";
import { BookingDialog } from "./BookingDialog";

beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function showModal() {
    this.open = true;
  };
  HTMLDialogElement.prototype.close = function close() {
    this.open = false;
  };
});

describe("BookingDialog", () => {
  it("mantiene Meet como opt-in y conserva la elección ante un refetch", async () => {
    const user = userEvent.setup();
    const gateway = new InMemorySchedulingGateway();
    const branch = gateway.branches[0]!;
    const service = gateway.services[0]!;
    const onCreate = vi.fn().mockResolvedValue(undefined);
    const props = {
      open: true,
      draft: {
        startAt: "2026-08-04T12:00:00.000Z",
        endAt: "2026-08-04T12:45:00.000Z",
      },
      booking: null,
      branchId: branch.id,
      timezone: branch.timezone,
      branches: gateway.branches,
      services: gateway.services,
      resources: gateway.resources,
      preferredServiceId: service.id,
      pending: false,
      onClose: vi.fn(),
      onCreate,
      onReschedule: vi.fn().mockResolvedValue(undefined),
    };
    const { rerender } = render(<BookingDialog {...props} />);
    const meet = screen.getByRole("checkbox", { name: /Google Meet/ });
    expect(meet).not.toBeChecked();

    await user.click(meet);
    expect(meet).toBeChecked();
    rerender(
      <BookingDialog
        {...props}
        services={gateway.services.map((item) => ({ ...item }))}
      />,
    );
    expect(screen.getByRole("checkbox", { name: /Google Meet/ })).toBeChecked();

    await user.type(screen.getByLabelText("Cliente"), "Cliente Meet");
    await user.click(screen.getByRole("button", { name: "Crear turno" }));
    await waitFor(() => expect(onCreate).toHaveBeenCalledOnce());
    expect(onCreate.mock.calls[0]![0]).toMatchObject({
      service_id: service.id,
      meet_requested: true,
    });
  });
});
