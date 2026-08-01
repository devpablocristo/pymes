import { DateTime } from "luxon";
import { describe, expect, it } from "vitest";
import { InMemorySchedulingGateway } from "./fakeSchedulingGateway";
import { SchedulingApiError } from "./errors";

const identity = { organizationId: "org_e2e", getToken: async () => null };

describe("InMemorySchedulingGateway", () => {
  it("mantiene el aislamiento tenant", async () => {
    const gateway = new InMemorySchedulingGateway();
    await expect(
      gateway.listBranches({ organizationId: "otra-organizacion", getToken: async () => null }),
    ).rejects.toMatchObject({ code: "FORBIDDEN" });
  });

  it("rechaza una doble reserva del mismo recurso", async () => {
    const gateway = new InMemorySchedulingGateway();
    const current = gateway.bookings[0]!;
    await expect(
      gateway.createBooking(identity, {
        branch_id: current.branch_id,
        service_id: current.service_id,
        customer: { name: "Reserva simultánea" },
        start_at: current.start_at,
        participants: 1,
        allocations: current.allocations,
      }),
    ).rejects.toMatchObject({ code: "RESOURCE_CONFLICT" });
  });

  it("versiona la reprogramación sin editar el turno original", async () => {
    const gateway = new InMemorySchedulingGateway();
    const current = gateway.bookings[0]!;
    const nextStart = DateTime.fromISO(current.start_at).plus({ days: 2 }).toISO()!;
    const replacement = await gateway.rescheduleBooking(
      identity,
      current.id,
      current.version,
      nextStart,
      60,
      current.allocations,
    );
    expect(current.status).toBe("rescheduled");
    expect(replacement.id).not.toBe(current.id);
    expect(replacement.supersedes_id).toBe(current.id);
    expect(replacement.duration_minutes).toBe(60);
  });

  it("edita sólo datos operativos y rechaza versiones obsoletas", async () => {
    const gateway = new InMemorySchedulingGateway();
    const current = gateway.bookings[1]!;
    const immutable = {
      branch_id: current.branch_id,
      service_id: current.service_id,
      start_at: current.start_at,
      end_at: current.end_at,
      status: current.status,
      service_name: current.service_name,
      price: current.price,
      currency: current.currency,
      duration_minutes: current.duration_minutes,
      timezone: current.timezone,
    };
    const updated = await gateway.updateBooking(identity, current.id, {
      expected_version: current.version,
      customer: {
        party_id: current.party_id,
        name: "Ada",
        email: "ada@example.com",
        phone: "+541155555555",
      },
      participants: 2,
      notes: "Acceso por recepción",
      substate_code: "first_visit",
    });
    expect(updated).toMatchObject({
      ...immutable,
      participants: 2,
      notes: "Acceso por recepción",
      substate_code: "first_visit",
      customer_email: "ada@example.com",
      customer_phone: "+541155555555",
      version: 2,
    });
    await expect(
      gateway.updateBooking(identity, current.id, {
        expected_version: 1,
        notes: "Versión obsoleta",
      }),
    ).rejects.toMatchObject({ code: "BOOKING_VERSION_CONFLICT" });
  });

  it("devuelve slots con una asignación elegible cuando el profesional es opcional", async () => {
    const gateway = new InMemorySchedulingGateway();
    const branch = gateway.branches[0]!;
    const service = gateway.services[0]!;
    const day = DateTime.now().plus({ days: 3 }).setZone(branch.timezone);
    const slots = await gateway.calculatePublicAvailability("centro-norte", {
      branch_id: branch.id,
      service_id: service.id,
      from: day.startOf("day").toUTC().toISO()!,
      until: day.endOf("day").toUTC().toISO()!,
      participants: 1,
    });
    expect(slots.length).toBeGreaterThan(0);
    expect(slots[0]!.allocations).toHaveLength(1);
  });

  it("no expone parties ni contactos en el catálogo público", async () => {
    const gateway = new InMemorySchedulingGateway();
    const catalog = await gateway.getPublicCatalog("centro-norte");
    expect(JSON.stringify(catalog)).not.toMatch(/party|email|phone/i);
  });

  it("decide el estado público desde la configuración del servicio", async () => {
    const gateway = new InMemorySchedulingGateway();
    const branch = gateway.branches[0]!;
    const service = gateway.services.find(
      (candidate) => candidate.confirmation_required,
    )!;
    const day = DateTime.now().plus({ days: 4 }).setZone(branch.timezone);
    const slots = await gateway.calculatePublicAvailability("centro-norte", {
      branch_id: branch.id,
      service_id: service.id,
      from: day.startOf("day").toUTC().toISO()!,
      until: day.endOf("day").toUTC().toISO()!,
      participants: 1,
    });

    const created = await gateway.createPublicBooking("centro-norte", {
      branch_id: branch.id,
      service_id: service.id,
      customer: { name: "Cliente público" },
      start_at: slots[0]!.start_at,
      participants: 1,
      allocations: slots[0]!.allocations,
    });

    expect(created[0]!.status).toBe("pending_confirmation");
  });

  it("clasifica los tokens públicos inválidos", async () => {
    const gateway = new InMemorySchedulingGateway();
    await expect(
      gateway.consumePublicAction("corto", { purpose: "confirm", expected_version: 1 }),
    ).rejects.toEqual(expect.objectContaining<Partial<SchedulingApiError>>({ code: "ACTION_TOKEN_INVALID" }));
  });
});
