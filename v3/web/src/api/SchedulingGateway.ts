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

export type RequestIdentity = {
  organizationId: string;
  getToken: () => Promise<string | null>;
};

export interface SchedulingGateway {
  listBranches(identity: RequestIdentity): Promise<Branch[]>;
  listServices(identity: RequestIdentity): Promise<Service[]>;
  listResources(identity: RequestIdentity, branchId?: string): Promise<Resource[]>;
  listBookings(identity: RequestIdentity, branchId: string, range: DateRange): Promise<Booking[]>;
  createBooking(identity: RequestIdentity, input: BookingInput): Promise<Booking[]>;
  updateBooking(
    identity: RequestIdentity,
    bookingId: string,
    input: BookingUpdateInput,
  ): Promise<Booking>;
  rescheduleBooking(
    identity: RequestIdentity,
    bookingId: string,
    expectedVersion: number,
    startAt: string,
    durationMinutes: number,
    allocations?: Booking["allocations"],
  ): Promise<Booking>;
  transitionBooking(
    identity: RequestIdentity,
    bookingId: string,
    action: BookingAction,
    expectedVersion: number,
    reason?: string,
  ): Promise<Booking>;
  calculateAvailability(identity: RequestIdentity, query: AvailabilityQuery): Promise<Slot[]>;
  listAvailabilityRules(identity: RequestIdentity, branchId: string): Promise<AvailabilityRule[]>;
  listBlocks(identity: RequestIdentity, branchId: string, range: DateRange): Promise<AvailabilityBlock[]>;
  createBlock(identity: RequestIdentity, input: AvailabilityBlockInput): Promise<AvailabilityBlock>;
  listWaitlist(identity: RequestIdentity, branchId: string): Promise<WaitlistEntry[]>;
  createWaitlistEntry(identity: RequestIdentity, input: WaitlistInput): Promise<WaitlistEntry>;
  listQueue(identity: RequestIdentity, branchId: string): Promise<QueueTicket[]>;
  createQueueTicket(identity: RequestIdentity, input: QueueTicketInput): Promise<QueueTicket>;
  advanceQueueTicket(
    identity: RequestIdentity,
    ticketId: string,
    input: QueueAdvanceInput,
  ): Promise<QueueTicket>;
  getPublicCatalog(organizationSlug: string): Promise<PublicCatalog>;
  calculatePublicAvailability(organizationSlug: string, query: AvailabilityQuery): Promise<Slot[]>;
  createPublicBooking(organizationSlug: string, input: PublicBookingInput): Promise<PublicBooking[]>;
  createPublicWaitlistEntry(organizationSlug: string, input: WaitlistInput): Promise<PublicWaitlistEntry>;
  consumePublicAction(token: string, input: PublicActionInput): Promise<void>;
}

export function newIdempotencyKey(prefix: string, sourceID?: string): string {
  return `${prefix}:${sourceID || crypto.randomUUID()}`;
}
