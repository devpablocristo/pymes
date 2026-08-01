import type { IncomingMessage, ServerResponse } from "node:http";
import {
  FiscalError,
  type FiscalProblem,
  type FiscalRequest,
} from "../../usecases/domain/fiscal.js";
import type { FiscalRequestDTO } from "../models/http.js";

export async function readFiscalRequest(
  request: IncomingMessage,
  optional: boolean,
): Promise<FiscalRequest | undefined> {
  const dto = await readJSON<FiscalRequestDTO>(request, optional);
  return dto === undefined ? undefined : structuredClone(dto);
}

export async function readJSON<T>(
  request: IncomingMessage,
  optional = false,
): Promise<T | undefined> {
  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of request) {
    const bytes = Buffer.from(chunk);
    size += bytes.length;
    if (size > 1024 * 1024) throw new FiscalError("VALIDATION_ERROR");
    chunks.push(bytes);
  }
  if (size === 0 && optional) return undefined;
  if (size === 0) throw new FiscalError("VALIDATION_ERROR");
  try {
    return JSON.parse(Buffer.concat(chunks).toString("utf8")) as T;
  } catch {
    throw new FiscalError("VALIDATION_ERROR");
  }
}

export function header(
  request: IncomingMessage,
  name: string,
): string | undefined {
  const value = request.headers[name];
  return Array.isArray(value) ? value[0] : value;
}

export function respondProblem(
  response: ServerResponse,
  error: unknown,
  correlationId: string,
): void {
  const fiscal =
    error instanceof FiscalError ? error : new FiscalError("INTERNAL_ERROR");
  respond(
    response,
    statusFor(fiscal.code),
    {
      code: fiscal.code,
      title: fiscal.message,
      correlation_id: correlationId,
    } satisfies FiscalProblem,
    "application/problem+json",
  );
}

export function respond(
  response: ServerResponse,
  status: number,
  value: unknown,
  contentType = "application/json",
): void {
  response.writeHead(status, { "content-type": contentType });
  response.end(JSON.stringify(value));
}

function statusFor(code: FiscalProblem["code"]): number {
  if (code === "UNAUTHORIZED_SERVICE") return 401;
  if (code === "IDEMPOTENCY_KEY_REUSED") return 409;
  if (code === "AUTHORITY_TIMEOUT") return 503;
  if (code === "INTERNAL_ERROR") return 500;
  return 422;
}
