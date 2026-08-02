import { DateTime } from "luxon";
import type {
  AvailabilityBlock,
  AvailabilityBlockInput,
  AvailabilityQuery,
  AvailabilityRule,
  Booking,
  BookingAction,
  BookingInput,
  BookingUpdateInput,
  Branch,
  DateRange,
  PublicActionInput,
  PublicBooking,
  PublicBookingInput,
  PublicCatalog,
  PublicWaitlistEntry,
  QueueAdvanceInput,
  QueueTicket,
  QueueTicketInput,
  Resource,
  Service,
  Slot,
  WaitlistEntry,
  WaitlistInput,
} from "../domain/scheduling";
import type { RequestIdentity, SchedulingGateway } from "./SchedulingGateway";
import { SchedulingApiError } from "./errors";

const ids = {
  branchCentro: "11111111-1111-4111-8111-111111111111",
  branchNorte: "22222222-2222-4222-8222-222222222222",
  serviceConsulta: "33333333-3333-4333-8333-333333333333",
  serviceColor: "44444444-4444-4444-8444-444444444444",
  professionalAna: "55555555-5555-4555-8555-555555555555",
  professionalLeo: "66666666-6666-4666-8666-666666666666",
  roomOne: "77777777-7777-4777-8777-777777777777",
  bookingOne: "88888888-8888-4888-8888-888888888888",
  bookingTwo: "99999999-9999-4999-8999-999999999999",
};

function nowInBuenosAires(): DateTime {
  return DateTime.now().setZone("America/Argentina/Buenos_Aires");
}

function stamp(): string {
  return DateTime.utc().toISO() ?? new Date().toISOString();
}

function deepCopy<T>(value: T): T {
  return structuredClone(value);
}

function bookingAt(
  id: string,
  service: Service,
  branchId: string,
  partyId: string,
  start: DateTime,
  resourceId: string,
  status: Booking["status"],
): Booking {
  return {
    id,
    branch_id: branchId,
    service_id: service.id,
    party_id: partyId,
    status,
    participants: 1,
    start_at: start.toUTC().toISO() ?? start.toJSDate().toISOString(),
    end_at: start.plus({ minutes: service.duration_minutes }).toUTC().toISO() ?? start.toJSDate().toISOString(),
    version: 1,
    service_name: service.name,
    price: service.price,
    currency: service.currency,
    duration_minutes: service.duration_minutes,
    timezone: "America/Argentina/Buenos_Aires",
    meet_requested: false,
    allocations: [{ resource_id: resourceId, mode: "exclusive", units: 1 }],
  };
}

export class InMemorySchedulingGateway implements SchedulingGateway {
  readonly branches: Branch[];
  readonly services: Service[];
  readonly resources: Resource[];
  readonly bookings: Booking[];
  readonly rules: AvailabilityRule[];
  readonly blocks: AvailabilityBlock[];
  readonly waitlist: WaitlistEntry[];
  readonly queue: QueueTicket[];
  private nextRescheduleError: SchedulingApiError | null = null;

  constructor() {
    const createdAt = stamp();
    this.branches = [
      {
        id: ids.branchCentro,
        organization_id: "org_e2e",
        code: "CTR",
        slug: "centro",
        name: "Centro",
        timezone: "America/Argentina/Buenos_Aires",
        address: "Av. Corrientes 1848",
        active: true,
        created_at: createdAt,
        updated_at: createdAt,
      },
      {
        id: ids.branchNorte,
        organization_id: "org_e2e",
        code: "NOR",
        slug: "norte",
        name: "Norte",
        timezone: "America/Argentina/Buenos_Aires",
        address: "Av. Maipú 860",
        active: true,
        created_at: createdAt,
        updated_at: createdAt,
      },
    ];
    this.services = [
      {
        id: ids.serviceConsulta,
        organization_id: "org_e2e",
        code: "CONS",
        name: "Consulta inicial",
        description: "Encuentro de diagnóstico y próximos pasos.",
        duration_minutes: 45,
        buffer_before_minutes: 5,
        buffer_after_minutes: 10,
        slot_minutes: 15,
        price: "18000.00",
        currency: "ARS",
        fulfillment_mode: "hybrid",
        max_participants: 1,
        allow_group: false,
        allow_waitlist: true,
        confirmation_required: true,
        active: true,
        resource_requirements: [
          {
            resource_kind: "professional",
            allocation_mode: "exclusive",
            units: 1,
            optional: false,
          },
        ],
        created_at: createdAt,
        updated_at: createdAt,
      },
      {
        id: ids.serviceColor,
        organization_id: "org_e2e",
        code: "COLOR",
        name: "Servicio extendido",
        description: "Atención con sala y profesional durante dos horas.",
        duration_minutes: 120,
        buffer_before_minutes: 10,
        buffer_after_minutes: 15,
        slot_minutes: 30,
        price: "42000.00",
        currency: "ARS",
        fulfillment_mode: "in_person",
        max_participants: 4,
        allow_group: true,
        allow_waitlist: true,
        confirmation_required: false,
        active: true,
        resource_requirements: [
          {
            resource_kind: "professional",
            allocation_mode: "exclusive",
            units: 1,
            optional: false,
          },
          {
            resource_kind: "room",
            allocation_mode: "exclusive",
            units: 1,
            optional: false,
          },
        ],
        created_at: createdAt,
        updated_at: createdAt,
      },
    ];
    this.resources = [
      {
        id: ids.professionalAna,
        organization_id: "org_e2e",
        branch_id: ids.branchCentro,
        code: "ANA",
        name: "Ana Torres",
        kind: "professional",
        capacity: 1,
        timezone: "America/Argentina/Buenos_Aires",
        active: true,
        created_at: createdAt,
        updated_at: createdAt,
      },
      {
        id: ids.professionalLeo,
        organization_id: "org_e2e",
        branch_id: ids.branchCentro,
        code: "LEO",
        name: "Leo Ruiz",
        kind: "professional",
        capacity: 1,
        timezone: "America/Argentina/Buenos_Aires",
        active: true,
        created_at: createdAt,
        updated_at: createdAt,
      },
      {
        id: ids.roomOne,
        organization_id: "org_e2e",
        branch_id: ids.branchCentro,
        code: "SALA-1",
        name: "Sala calma",
        kind: "room",
        capacity: 4,
        timezone: "America/Argentina/Buenos_Aires",
        active: true,
        created_at: createdAt,
        updated_at: createdAt,
      },
    ];
    const day = nowInBuenosAires().plus({ days: 1 }).startOf("day");
    this.bookings = [
      bookingAt(
        ids.bookingOne,
        this.services[0]!,
        ids.branchCentro,
        "party-clara",
        day.set({ hour: 9, minute: 30 }),
        ids.professionalAna,
        "confirmed",
      ),
      bookingAt(
        ids.bookingTwo,
        this.services[1]!,
        ids.branchCentro,
        "party-tomas",
        day.set({ hour: 11, minute: 0 }),
        ids.professionalLeo,
        "pending_confirmation",
      ),
    ];
    this.bookings[0]!.customer_name = "Clara Méndez";
    this.bookings[0]!.customer_email = "clara@example.invalid";
    this.bookings[1]!.customer_name = "Tomás Ruiz";
    this.rules = [
      {
        id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
        organization_id: "org_e2e",
        branch_id: ids.branchCentro,
        kind: "branch",
        weekday: 1,
        start_minute: 480,
        end_minute: 1080,
        timezone: "America/Argentina/Buenos_Aires",
        active: true,
      },
    ];
    this.blocks = [];
    this.waitlist = [
      {
        id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
        branch_id: ids.branchCentro,
        service_id: ids.serviceConsulta,
        customer_name: "Marina Costa",
        customer_email: "marina@example.invalid",
        preferred_from: day.toUTC().toISO()!,
        preferred_until: day.endOf("day").toUTC().toISO()!,
        participants: 1,
        meet_requested: false,
        party_id: "party-marina",
        status: "pending",
        version: 1,
      },
    ];
    this.queue = [
      {
        id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
        branch_id: ids.branchCentro,
        service_id: ids.serviceConsulta,
        party_id: "party-diego",
        priority: 0,
        number: 7,
        status: "waiting",
        version: 1,
      },
    ];
  }

  failNextReschedule(code: "SLOT_CONFLICT" | "RESOURCE_CONFLICT" | "BOOKING_VERSION_CONFLICT"): void {
    this.nextRescheduleError = new SchedulingApiError(code, code, 409);
  }

  private assertOrg(identity: RequestIdentity): void {
    if (identity.organizationId !== "org_e2e") {
      throw new SchedulingApiError("FORBIDDEN", "Tenant desconocido", 403);
    }
  }

  async listBranches(identity: RequestIdentity): Promise<Branch[]> {
    this.assertOrg(identity);
    return deepCopy(this.branches);
  }

  async listServices(identity: RequestIdentity): Promise<Service[]> {
    this.assertOrg(identity);
    return deepCopy(this.services);
  }

  async listResources(identity: RequestIdentity, branchId?: string): Promise<Resource[]> {
    this.assertOrg(identity);
    return deepCopy(branchId ? this.resources.filter((item) => item.branch_id === branchId) : this.resources);
  }

  async listBookings(identity: RequestIdentity, branchId: string, range: DateRange): Promise<Booking[]> {
    this.assertOrg(identity);
    return deepCopy(
      this.bookings.filter(
        (item) => item.branch_id === branchId && item.start_at < range.until && item.end_at > range.from,
      ),
    );
  }

  async createBooking(identity: RequestIdentity, input: BookingInput): Promise<Booking[]> {
    this.assertOrg(identity);
    const service = this.services.find((item) => item.id === input.service_id);
    if (!service) {
      throw new SchedulingApiError("NOT_FOUND", "Servicio inexistente", 404);
    }
    const start = DateTime.fromISO(input.start_at, { setZone: true });
    const allocations = input.allocations ?? [];
    this.assertNoConflict(start, service.duration_minutes, allocations);
    const booking = bookingAt(
      crypto.randomUUID(),
      service,
      input.branch_id,
      input.customer.party_id ?? `party-${crypto.randomUUID()}`,
      start,
      allocations[0]?.resource_id ?? ids.professionalAna,
      input.status ?? "pending_confirmation",
    );
    booking.participants = input.participants;
    booking.meet_requested = input.meet_requested ?? false;
    booking.allocations = deepCopy(allocations);
    booking.customer_name = input.customer.name;
    if (input.customer.email) booking.customer_email = input.customer.email;
    if (input.customer.phone) booking.customer_phone = input.customer.phone;
    if (input.notes) booking.notes = input.notes;
    this.bookings.push(booking);
    return [deepCopy(booking)];
  }

  async updateBooking(
    identity: RequestIdentity,
    bookingId: string,
    input: BookingUpdateInput,
  ): Promise<Booking> {
    this.assertOrg(identity);
    const booking = this.bookings.find((item) => item.id === bookingId);
    if (!booking) {
      throw new SchedulingApiError("NOT_FOUND", "Turno inexistente", 404);
    }
    if (booking.version !== input.expected_version) {
      throw new SchedulingApiError("BOOKING_VERSION_CONFLICT", "Versión desactualizada", 409);
    }
    if (
      input.customer === undefined &&
      input.participants === undefined &&
      input.notes === undefined &&
      input.substate_code === undefined
    ) {
      throw new SchedulingApiError("VALIDATION_ERROR", "No hay campos editables", 400);
    }
    if (input.participants !== undefined) {
      const service = this.services.find((item) => item.id === booking.service_id);
      if (!service || input.participants < 1 || input.participants > service.max_participants) {
        throw new SchedulingApiError("CAPACITY_EXCEEDED", "Capacidad no disponible", 409);
      }
      if (
        input.participants !== booking.participants &&
        !["held", "pending_confirmation", "confirmed", "checked_in"].includes(booking.status)
      ) {
        throw new SchedulingApiError("BOOKING_STATE_INVALID", "El turno ya no admite participantes", 409);
      }
      booking.participants = input.participants;
    }
    if (input.customer) {
      booking.party_id = input.customer.party_id ?? `party-${crypto.randomUUID()}`;
      booking.customer_name = input.customer.name;
      booking.customer_email = input.customer.email;
      booking.customer_phone = input.customer.phone;
    }
    if (input.notes !== undefined) booking.notes = input.notes;
    if (input.substate_code !== undefined) booking.substate_code = input.substate_code || undefined;
    booking.version += 1;
    return deepCopy(booking);
  }

  async rescheduleBooking(
    identity: RequestIdentity,
    bookingId: string,
    expectedVersion: number,
    startAt: string,
    durationMinutes: number,
    allocations?: Booking["allocations"],
  ): Promise<Booking> {
    this.assertOrg(identity);
    if (this.nextRescheduleError) {
      const error = this.nextRescheduleError;
      this.nextRescheduleError = null;
      throw error;
    }
    const current = this.bookings.find((item) => item.id === bookingId);
    if (!current) {
      throw new SchedulingApiError("NOT_FOUND", "Turno inexistente", 404);
    }
    if (current.version !== expectedVersion) {
      throw new SchedulingApiError("BOOKING_VERSION_CONFLICT", "Versión desactualizada", 409);
    }
    const nextAllocations = allocations ?? current.allocations;
    const start = DateTime.fromISO(startAt, { setZone: true });
    this.assertNoConflict(start, durationMinutes, nextAllocations, bookingId);
    const replacement = {
      ...current,
      id: crypto.randomUUID(),
      supersedes_id: current.id,
      start_at: start.toUTC().toISO()!,
      end_at: start.plus({ minutes: durationMinutes }).toUTC().toISO()!,
      duration_minutes: durationMinutes,
      allocations: deepCopy(nextAllocations),
      version: 1,
      status: "confirmed" as const,
    };
    current.status = "rescheduled";
    current.version += 1;
    this.bookings.push(replacement);
    return deepCopy(replacement);
  }

  async transitionBooking(
    identity: RequestIdentity,
    bookingId: string,
    action: BookingAction,
    expectedVersion: number,
    _reason?: string,
  ): Promise<Booking> {
    this.assertOrg(identity);
    const booking = this.bookings.find((item) => item.id === bookingId);
    if (!booking) {
      throw new SchedulingApiError("NOT_FOUND", "Turno inexistente", 404);
    }
    if (booking.version !== expectedVersion) {
      throw new SchedulingApiError("BOOKING_VERSION_CONFLICT", "Versión desactualizada", 409);
    }
    const statusByAction: Record<BookingAction, Booking["status"]> = {
      confirm: "confirmed",
      cancel: "cancelled",
      "check-in": "checked_in",
      complete: "completed",
      "no-show": "no_show",
    };
    booking.status = statusByAction[action];
    booking.version += 1;
    return deepCopy(booking);
  }

  async calculateAvailability(identity: RequestIdentity, query: AvailabilityQuery): Promise<Slot[]> {
    this.assertOrg(identity);
    return this.slots(query);
  }

  async listAvailabilityRules(identity: RequestIdentity, branchId: string): Promise<AvailabilityRule[]> {
    this.assertOrg(identity);
    return deepCopy(this.rules.filter((item) => item.branch_id === branchId));
  }

  async listBlocks(identity: RequestIdentity, branchId: string, range: DateRange): Promise<AvailabilityBlock[]> {
    this.assertOrg(identity);
    return deepCopy(
      this.blocks.filter(
        (item) => item.branch_id === branchId && item.start_at < range.until && item.end_at > range.from,
      ),
    );
  }

  async createBlock(identity: RequestIdentity, input: AvailabilityBlockInput): Promise<AvailabilityBlock> {
    this.assertOrg(identity);
    const block: AvailabilityBlock = {
      ...input,
      id: input.id ?? crypto.randomUUID(),
      organization_id: identity.organizationId,
    };
    this.blocks.push(block);
    return deepCopy(block);
  }

  async listWaitlist(identity: RequestIdentity, branchId: string): Promise<WaitlistEntry[]> {
    this.assertOrg(identity);
    return deepCopy(this.waitlist.filter((item) => item.branch_id === branchId));
  }

  async createWaitlistEntry(identity: RequestIdentity, input: WaitlistInput): Promise<WaitlistEntry> {
    this.assertOrg(identity);
    const item: WaitlistEntry = {
      ...input,
      id: input.id ?? crypto.randomUUID(),
      party_id: input.customer.party_id ?? `party-${crypto.randomUUID()}`,
      meet_requested: input.meet_requested ?? false,
      status: "pending",
      version: 1,
      customer_name: input.customer.name,
    };
    if (input.customer.email) item.customer_email = input.customer.email;
    if (input.customer.phone) item.customer_phone = input.customer.phone;
    this.waitlist.push(item);
    return deepCopy(item);
  }

  async listQueue(identity: RequestIdentity, branchId: string): Promise<QueueTicket[]> {
    this.assertOrg(identity);
    return deepCopy(this.queue.filter((item) => item.branch_id === branchId));
  }

  async createQueueTicket(identity: RequestIdentity, input: QueueTicketInput): Promise<QueueTicket> {
    this.assertOrg(identity);
    const ticket: QueueTicket = {
      ...input,
      id: input.id ?? crypto.randomUUID(),
      number: Math.max(0, ...this.queue.map((item) => item.number)) + 1,
      status: "waiting",
      version: 1,
    };
    this.queue.push(ticket);
    return deepCopy(ticket);
  }

  async advanceQueueTicket(
    identity: RequestIdentity,
    ticketId: string,
    input: QueueAdvanceInput,
  ): Promise<QueueTicket> {
    this.assertOrg(identity);
    const ticket = this.queue.find((item) => item.id === ticketId);
    if (!ticket) {
      throw new SchedulingApiError("NOT_FOUND", "Ticket inexistente", 404);
    }
    if (ticket.version !== input.expected_version) {
      throw new SchedulingApiError("BOOKING_VERSION_CONFLICT", "Versión desactualizada", 409);
    }
    ticket.status = input.status;
    ticket.version += 1;
    return deepCopy(ticket);
  }

  async getPublicCatalog(organizationSlug: string): Promise<PublicCatalog> {
    if (organizationSlug !== "centro-norte") {
      throw new SchedulingApiError("NOT_FOUND", "Agenda inexistente", 404);
    }
    return {
      branches: this.branches.map(({ id, slug, name, timezone, address }) => ({
        id,
        slug: slug ?? "",
        name,
        timezone,
        address: address ?? "",
      })),
      services: this.services.map(
        ({
          id,
          code,
          name,
          description,
          duration_minutes,
          buffer_before_minutes,
          buffer_after_minutes,
          slot_minutes,
          price,
          currency,
          fulfillment_mode,
          max_participants,
          allow_group,
          allow_waitlist,
          confirmation_required,
        }) => ({
          id,
          code,
          name,
          description: description ?? "",
          duration_minutes,
          buffer_before_minutes: buffer_before_minutes ?? 0,
          buffer_after_minutes: buffer_after_minutes ?? 0,
          slot_minutes,
          price,
          currency,
          fulfillment_mode,
          max_participants,
          allow_group: allow_group ?? false,
          allow_waitlist: allow_waitlist ?? false,
          confirmation_required: confirmation_required ?? false,
        }),
      ),
      resources: this.resources.map(({ id, branch_id, name, kind, capacity, timezone }) => ({
        id,
        branch_id,
        name,
        kind,
        capacity,
        timezone,
      })),
    };
  }

  async calculatePublicAvailability(organizationSlug: string, query: AvailabilityQuery): Promise<Slot[]> {
    await this.getPublicCatalog(organizationSlug);
    return this.slots(query);
  }

  async createPublicBooking(
    organizationSlug: string,
    input: PublicBookingInput,
  ): Promise<PublicBooking[]> {
    await this.getPublicCatalog(organizationSlug);
    const service = this.services.find((candidate) => candidate.id === input.service_id);
    if (!service) {
      throw new SchedulingApiError("NOT_FOUND", "Servicio inexistente", 404);
    }
    const created = await this.createBooking(
      { organizationId: "org_e2e", getToken: async () => null },
      {
        ...input,
        status: service.confirmation_required
          ? "pending_confirmation"
          : "confirmed",
      },
    );
    return created.map(
      ({
        id,
        series_id,
        session_id,
        supersedes_id,
        branch_id,
        service_id,
        status,
        participants,
        start_at,
        end_at,
        version,
        service_name,
        price,
        currency,
        duration_minutes,
        timezone,
        meet_requested,
      }) => ({
        id,
        ...(series_id ? { series_id } : {}),
        ...(session_id ? { session_id } : {}),
        ...(supersedes_id ? { supersedes_id } : {}),
        branch_id,
        service_id,
        status,
        participants,
        start_at,
        end_at,
        version,
        service_name,
        price,
        currency,
        duration_minutes,
        timezone,
        meet_requested,
      }),
    );
  }

  async createPublicWaitlistEntry(
    organizationSlug: string,
    input: WaitlistInput,
  ): Promise<PublicWaitlistEntry> {
    await this.getPublicCatalog(organizationSlug);
    const created = await this.createWaitlistEntry(
      { organizationId: "org_e2e", getToken: async () => null },
      input,
    );
    return {
      id: created.id,
      branch_id: created.branch_id,
      service_id: created.service_id,
      preferred_from: created.preferred_from,
      preferred_until: created.preferred_until,
      participants: created.participants,
      meet_requested: created.meet_requested,
      status: created.status,
      version: created.version,
    };
  }

  async consumePublicAction(token: string, input: PublicActionInput): Promise<void> {
    if (token.includes("expired")) {
      throw new SchedulingApiError("ACTION_TOKEN_EXPIRED", "Token vencido", 410);
    }
    if (token.length < 80) {
      throw new SchedulingApiError("ACTION_TOKEN_INVALID", "Token inválido", 401);
    }
    if (!input.purpose) {
      throw new SchedulingApiError("VALIDATION_ERROR", "Falta propósito", 400);
    }
  }

  private slots(query: AvailabilityQuery): Slot[] {
    const service = this.services.find((item) => item.id === query.service_id);
    if (!service) {
      throw new SchedulingApiError("NOT_FOUND", "Servicio inexistente", 404);
    }
    const from = DateTime.fromISO(query.from, { setZone: true }).setZone("America/Argentina/Buenos_Aires");
    const until = DateTime.fromISO(query.until, { setZone: true }).setZone("America/Argentina/Buenos_Aires");
    const candidateResources =
      query.allocations && query.allocations.length > 0
        ? query.allocations
        : [
            {
              resource_id: this.resources.find(
                (item) => item.branch_id === query.branch_id && item.kind === "professional",
              )?.id ?? ids.professionalAna,
              mode: "exclusive" as const,
              units: 1,
            },
          ];
    const slots: Slot[] = [];
    let cursor = from.startOf("day").set({ hour: 8 });
    while (cursor < until && slots.length < 80) {
      const end = cursor.plus({ minutes: service.duration_minutes });
      if (
        cursor >= from &&
        end <= until &&
        cursor.hour < 18 &&
        !this.hasConflict(cursor, service.duration_minutes, candidateResources)
      ) {
        slots.push({
          start_at: cursor.toUTC().toISO()!,
          end_at: end.toUTC().toISO()!,
          occupies_from: cursor.minus({ minutes: service.buffer_before_minutes ?? 0 }).toUTC().toISO()!,
          occupies_until: end.plus({ minutes: service.buffer_after_minutes ?? 0 }).toUTC().toISO()!,
          timezone: "America/Argentina/Buenos_Aires",
          allocations: deepCopy(candidateResources),
          remaining: Math.max(1, service.max_participants - query.participants + 1),
        });
      }
      cursor = cursor.plus({ minutes: service.slot_minutes });
      if (cursor.hour >= 18) {
        cursor = cursor.plus({ days: 1 }).startOf("day").set({ hour: 8 });
      }
    }
    return slots;
  }

  private hasConflict(
    start: DateTime,
    durationMinutes: number,
    allocations: Booking["allocations"],
    ignoredBookingId?: string,
  ): boolean {
    const end = start.plus({ minutes: durationMinutes });
    const resourceIds = new Set(allocations.map((item) => item.resource_id));
    return this.bookings.some((booking) => {
      if (booking.id === ignoredBookingId || ["cancelled", "rescheduled"].includes(booking.status)) {
        return false;
      }
      const sharesResource = booking.allocations.some((allocation) => resourceIds.has(allocation.resource_id));
      const bookingStart = DateTime.fromISO(booking.start_at);
      const bookingEnd = DateTime.fromISO(booking.end_at);
      return sharesResource && start < bookingEnd && end > bookingStart;
    });
  }

  private assertNoConflict(
    start: DateTime,
    durationMinutes: number,
    allocations: Booking["allocations"],
    ignoredBookingId?: string,
  ): void {
    if (this.hasConflict(start, durationMinutes, allocations, ignoredBookingId)) {
      throw new SchedulingApiError("RESOURCE_CONFLICT", "Recurso ocupado", 409);
    }
  }
}

let singleton: InMemorySchedulingGateway | undefined;

export function getFakeSchedulingGateway(): InMemorySchedulingGateway {
  singleton ??= new InMemorySchedulingGateway();
  return singleton;
}
