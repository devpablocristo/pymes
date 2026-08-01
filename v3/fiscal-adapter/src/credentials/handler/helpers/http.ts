import type { IncomingMessage, ServerResponse } from "node:http";
import { CredentialError } from "../../usecases/domain/credential.js";
import { FiscalError } from "../../../fiscal/usecases/domain/fiscal.js";
import type {
  CSRResult,
} from "../../usecases.js";
import type {
  CredentialMetadata,
  PointOfSale,
} from "../../usecases/domain/credential.js";
import type {
  CredentialDTO,
  CSRResultDTO,
  PointOfSaleResultDTO,
} from "../models/http.js";

const MAX_BODY_BYTES = 1024 * 1024;

export async function readJSON<T>(request: IncomingMessage): Promise<T> {
  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of request) {
    const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
    size += buffer.byteLength;
    if (size > MAX_BODY_BYTES) {
      throw new CredentialError("VALIDATION_ERROR", "request body too large");
    }
    chunks.push(buffer);
  }
  try {
    return JSON.parse(Buffer.concat(chunks).toString("utf8")) as T;
  } catch {
    throw new CredentialError("VALIDATION_ERROR", "invalid JSON");
  }
}

export function header(
  request: IncomingMessage,
  name: string,
): string | undefined {
  const value = request.headers[name];
  return Array.isArray(value) ? value[0] : value;
}

export function respond(
  response: ServerResponse,
  status: number,
  body: unknown,
): void {
  response.writeHead(status, { "content-type": "application/json" });
  response.end(JSON.stringify(body));
}

export function respondProblem(
  response: ServerResponse,
  error: unknown,
  correlationId: string,
): void {
  if (error instanceof CredentialError) {
    const status =
      error.code === "CREDENTIAL_NOT_FOUND"
        ? 404
        : error.code === "CREDENTIAL_VERSION_CONFLICT" ||
            error.code === "IDEMPOTENCY_KEY_REUSED"
          ? 409
          : 422;
    respond(response, status, {
      code: error.code,
      title: error.code,
      correlation_id: correlationId,
    });
    return;
  }
  if (error instanceof FiscalError) {
    const status =
      error.code === "UNAUTHORIZED_SERVICE"
        ? 401
        : error.code === "AUTHORITY_TIMEOUT"
          ? 503
          : 422;
    respond(response, status, {
      code: error.code,
      title: error.code,
      correlation_id: correlationId,
    });
    return;
  }
  respond(response, 500, {
    code: "INTERNAL_ERROR",
    title: "INTERNAL_ERROR",
    correlation_id: correlationId,
  });
}

export function credentialDTO(
  value: CredentialMetadata,
): CredentialDTO {
  return {
    id: value.id,
    organization_id: value.organizationId,
    cuit: value.cuit,
    environment: value.environment,
    legal_name: value.legalName,
    common_name: value.commonName,
    status: value.status,
    ...(value.certificateFingerprint === undefined
      ? {}
      : { certificate_fingerprint: value.certificateFingerprint }),
    ...(value.certificateValidFrom === undefined
      ? {}
      : { certificate_valid_from: value.certificateValidFrom }),
    ...(value.certificateExpiresAt === undefined
      ? {}
      : { certificate_expires_at: value.certificateExpiresAt }),
    ...(value.certificateSerialNumber === undefined
      ? {}
      : { certificate_serial_number: value.certificateSerialNumber }),
    version: value.version,
    created_at: value.createdAt,
    updated_at: value.updatedAt,
  };
}

export function csrResultDTO(value: CSRResult): CSRResultDTO {
  return {
    credential: credentialDTO(value.credential),
    csr_pem: value.csrPem,
  };
}

export function pointOfSaleDTO(
  value: PointOfSale,
): PointOfSaleResultDTO {
  return {
    organization_id: value.organizationId,
    credential_id: value.credentialId,
    environment: value.environment,
    number: value.number,
    enabled: value.enabled,
    ...(value.validatedAt === undefined
      ? {}
      : { validated_at: value.validatedAt }),
  };
}
