import type { SchedulingErrorCode } from "../domain/scheduling";

export class SchedulingApiError extends Error {
  readonly code: SchedulingErrorCode;
  readonly status: number;

  constructor(code: SchedulingErrorCode, message: string, status = 500) {
    super(message);
    this.name = "SchedulingApiError";
    this.code = code;
    this.status = status;
  }
}

export function errorMessage(error: unknown): string {
  if (!(error instanceof SchedulingApiError)) {
    return "No se pudo completar la acción. Revisá la conexión e intentá de nuevo.";
  }
  switch (error.code) {
    case "SLOT_CONFLICT":
    case "RESOURCE_CONFLICT":
      return "Ese horario acaba de ocuparse. Actualizamos la agenda para que elijas otro.";
    case "CAPACITY_EXCEEDED":
      return "La sesión ya no tiene cupo disponible.";
    case "BOOKING_VERSION_CONFLICT":
      return "El turno cambió en otra sesión. Recargamos su versión más reciente.";
    case "HOLD_EXPIRED":
      return "La reserva temporal venció. Elegí otro horario para continuar.";
    case "ACTION_TOKEN_EXPIRED":
      return "Este enlace venció. Pedí un enlace nuevo a la empresa.";
    case "ACTION_TOKEN_INVALID":
      return "El enlace no es válido o ya fue utilizado.";
    case "BOOKING_STATE_INVALID":
      return "El turno ya no admite esa acción.";
    case "FORBIDDEN":
      return "Tu rol no permite realizar esta acción.";
    case "VALIDATION_ERROR":
      return error.message || "Revisá los datos ingresados.";
    default:
      return error.message || "No se pudo completar la acción.";
  }
}
