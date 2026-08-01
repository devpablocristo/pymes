import type { components } from "../api/generated";

export type Branch = components["schemas"]["Branch"];
export type BranchInput = components["schemas"]["BranchInput"];
export type Service = components["schemas"]["Service"];
export type ServiceInput = components["schemas"]["ServiceInput"];
export type Resource = components["schemas"]["Resource"];
export type ResourceInput = components["schemas"]["ResourceInput"];
export type Allocation = components["schemas"]["Allocation"];
export type AvailabilityRule = components["schemas"]["AvailabilityRule"];
export type AvailabilityRuleInput = components["schemas"]["AvailabilityRuleInput"];
export type AvailabilityBlock = components["schemas"]["AvailabilityException"];
export type AvailabilityBlockInput = components["schemas"]["AvailabilityExceptionInput"];
export type AvailabilityQuery = components["schemas"]["AvailabilityQuery"];
export type Slot = components["schemas"]["Slot"];
export type Booking = components["schemas"]["Booking"];
export type PublicBooking = components["schemas"]["PublicBooking"];
export type BookingInput = components["schemas"]["BookingInput"];
export type BookingStatus = Booking["status"];
export type RescheduleInput = components["schemas"]["RescheduleInput"];
export type WaitlistEntry = components["schemas"]["WaitlistEntry"];
export type PublicWaitlistEntry = components["schemas"]["PublicWaitlistEntry"];
export type WaitlistInput = components["schemas"]["WaitlistInput"];
export type QueueTicket = components["schemas"]["QueueTicket"];
export type QueueTicketInput = components["schemas"]["QueueTicketInput"];
export type QueueAdvanceInput = components["schemas"]["QueueAdvanceInput"];
export type PublicActionInput = components["schemas"]["PublicActionInput"];

export type SchedulingErrorCode =
  | "VALIDATION_ERROR"
  | "NOT_FOUND"
  | "SLOT_CONFLICT"
  | "RESOURCE_CONFLICT"
  | "CAPACITY_EXCEEDED"
  | "BOOKING_VERSION_CONFLICT"
  | "HOLD_EXPIRED"
  | "ACTION_TOKEN_INVALID"
  | "ACTION_TOKEN_EXPIRED"
  | "BOOKING_STATE_INVALID"
  | "IDEMPOTENCY_KEY_REUSED"
  | "FORBIDDEN"
  | "NETWORK_ERROR"
  | "UNKNOWN";

export type DateRange = {
  from: string;
  until: string;
};

export type PublicCatalog = components["schemas"]["PublicSchedulingCatalog"];
export type PublicBranch = components["schemas"]["PublicSchedulingBranch"];
export type PublicService = components["schemas"]["PublicSchedulingService"];
export type PublicResource = components["schemas"]["PublicSchedulingResource"];

export type BookingAction = "confirm" | "cancel" | "check-in" | "complete" | "no-show";

export const bookingStatusLabels: Record<BookingStatus, string> = {
  held: "En espera",
  pending_confirmation: "A confirmar",
  confirmed: "Confirmado",
  checked_in: "En recepción",
  completed: "Completado",
  cancelled: "Cancelado",
  rescheduled: "Reprogramado",
  no_show: "No asistió",
};

export function resourceLabel(resource: Resource): string {
  const kind: Record<Resource["kind"], string> = {
    professional: "Profesional",
    room: "Sala",
    machine: "Máquina",
    vehicle: "Vehículo",
    equipment: "Equipo",
    generic: "Recurso",
  };
  return `${kind[resource.kind]} · ${resource.name}`;
}
