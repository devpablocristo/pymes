import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { documentTypes, FiscalError, type FiscalProblem, type FiscalRequest, type FiscalResult } from "../domain/fiscal.js";
import type { RequestContext } from "../usecases/fiscal-service.js";
import type { InternalAuthorizer } from "../ports/internal-authorizer.js";
import type { FiscalRuntimeObserver } from "../ports/runtime-observer.js";

export interface FiscalApplication {
  authorize(request: FiscalRequest, context: RequestContext): Promise<FiscalResult>;
  consult(organizationId: string, requestId: string, request: FiscalRequest | undefined, context: RequestContext): Promise<FiscalResult>;
}

export function createFiscalHTTPServer(application: FiscalApplication, authorizer: InternalAuthorizer, runtime: FiscalRuntimeObserver) {
  return createServer(async (request, response) => {
    response.setHeader("cache-control", "no-store");
    if (request.method === "GET" && request.url === "/healthz") return respond(response, 200, { status: "ok" });
    if (request.method === "GET" && request.url === "/readyz") {
      try {
        await runtime.ping();
        return respond(response, 200, { status: "ready" });
      } catch {
        return respond(response, 503, { status: "not_ready" });
      }
    }
    if (request.method === "GET" && request.url === "/metrics") {
      try {
        const metrics = await runtime.metrics();
        response.writeHead(200, { "content-type": "text/plain; version=0.0.4" });
        response.end(
          `pymes_fiscal_results{status="authorized"} ${metrics.authorized}\n` +
          `pymes_fiscal_results{status="rejected"} ${metrics.rejected}\n` +
          `pymes_fiscal_results{status="uncertain"} ${metrics.uncertain}\n` +
          `pymes_fiscal_results{status="not_found"} ${metrics.not_found}\n`,
        );
        return;
      } catch {
        return respond(response, 503, { code: "METRICS_UNAVAILABLE" });
      }
    }

    const correlationId = header(request, "x-correlation-id") ?? header(request, "idempotency-key") ?? "missing-correlation-id";
    if (request.method === "GET" && request.url === "/internal/v1/catalogs/document-types") {
      try {
        await authorizer.authorize(header(request, "authorization"), "fiscal");
        return respond(response, 200, documentTypes.map((code) => ({
          code,
          letter: code.at(-1),
          kind: code.startsWith("NC") ? "credit_note" : code.startsWith("ND") ? "debit_note" : "invoice",
        })));
      } catch (error) {
        return respondProblem(response, error, correlationId);
      }
    }

    const match = request.url?.match(/^\/internal\/v1\/organizations\/([^/]+)\/authorizations(?:\/([^/]+)\/consult)?$/);
    if (request.method !== "POST" || match === undefined || match === null) return respond(response, 404, { code: "NOT_FOUND" });

    try {
      const organizationId = decodeURIComponent(match[1]);
      const requestId = match[2] === undefined ? undefined : decodeURIComponent(match[2]);
      await authorizer.authorize(header(request, "authorization"), "fiscal", organizationId);
      const idempotencyKey = header(request, "idempotency-key");
      if (idempotencyKey === undefined) throw new FiscalError("VALIDATION_ERROR");
      const context = { idempotencyKey, correlationId };
      const fiscalRequest = await readJSON<FiscalRequest>(request, requestId !== undefined);
      if (fiscalRequest !== undefined && fiscalRequest.organization_id !== organizationId) throw new FiscalError("VALIDATION_ERROR");

      if (requestId !== undefined) {
        const result = await application.consult(organizationId, requestId, fiscalRequest, context);
        return respond(response, 200, result);
      }
      if (fiscalRequest === undefined) throw new FiscalError("VALIDATION_ERROR");
      const result = await application.authorize(fiscalRequest, context);
      return respond(response, result.status === "uncertain" ? 202 : 201, result);
    } catch (error) {
      return respondProblem(response, error, correlationId);
    }
  });
}

async function readJSON<T>(request: IncomingMessage, optional: boolean): Promise<T | undefined> {
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

function header(request: IncomingMessage, name: string): string | undefined {
  const value = request.headers[name];
  return Array.isArray(value) ? value[0] : value;
}

function statusFor(code: FiscalProblem["code"]): number {
  if (code === "UNAUTHORIZED_SERVICE") return 401;
  if (code === "IDEMPOTENCY_KEY_REUSED") return 409;
  if (code === "AUTHORITY_TIMEOUT") return 503;
  if (code === "INTERNAL_ERROR") return 500;
  return 422;
}

function respondProblem(response: ServerResponse, error: unknown, correlationId: string): void {
  const fiscal = error instanceof FiscalError ? error : new FiscalError("INTERNAL_ERROR");
  respond(
    response,
    statusFor(fiscal.code),
    { code: fiscal.code, title: fiscal.message, correlation_id: correlationId } satisfies FiscalProblem,
    "application/problem+json",
  );
}

function respond(response: ServerResponse, status: number, value: unknown, contentType = "application/json"): void {
  response.writeHead(status, { "content-type": contentType });
  response.end(JSON.stringify(value));
}
