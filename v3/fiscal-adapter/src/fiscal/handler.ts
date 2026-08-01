import { createServer } from "node:http";
import {
  handleCredentialRequest,
  type CredentialApplication,
} from "../credentials/handler.js";
import {
  documentTypes,
  FiscalError,
  type FiscalRequest,
  type FiscalResult,
} from "./usecases/domain/fiscal.js";
import type { InternalIdentity, RequestContext } from "./usecases.js";
import type { DocumentTypeDTO } from "./handler/models/http.js";
import {
  header,
  readFiscalRequest,
  respond,
  respondProblem,
} from "./handler/helpers/http.js";

export interface InternalAuthorizer {
  authorize(
    authorization: string | undefined,
    audience: "fiscal",
    expectedOrganizationId?: string,
    expectedCorrelationId?: string,
  ): Promise<InternalIdentity>;
}

export interface FiscalRuntimeMetrics {
  authorized: number;
  rejected: number;
  uncertain: number;
  not_found: number;
}

export interface FiscalRuntimeObserver {
  ping(): Promise<void>;
  metrics(): Promise<FiscalRuntimeMetrics>;
}

export interface FiscalApplication {
  authorize(request: FiscalRequest, context: RequestContext): Promise<FiscalResult>;
  consult(organizationId: string, requestId: string, request: FiscalRequest | undefined, context: RequestContext): Promise<FiscalResult>;
}

export function createFiscalHTTPServer(
  application: FiscalApplication,
  authorizer: InternalAuthorizer,
  runtime: FiscalRuntimeObserver,
  credentials?: CredentialApplication,
) {
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

    if (
      credentials !== undefined &&
      (await handleCredentialRequest(
        request,
        response,
        credentials,
        authorizer,
      ))
    ) {
      return;
    }

    const correlationHeader = header(request, "x-correlation-id");
    const correlationId = correlationHeader ?? header(request, "idempotency-key") ?? "missing-correlation-id";
    if (request.method === "GET" && request.url === "/internal/v1/catalogs/document-types") {
      try {
        if (correlationHeader === undefined) throw new FiscalError("VALIDATION_ERROR");
        const identity = await authorizer.authorize(
          header(request, "authorization"),
          "fiscal",
          undefined,
          correlationHeader,
        );
        if (identity.correlationId !== correlationHeader) throw new FiscalError("UNAUTHORIZED_SERVICE");
        return respond(response, 200, documentTypes.map((code): DocumentTypeDTO => ({
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
      const idempotencyKey = header(request, "idempotency-key");
      if (idempotencyKey === undefined || correlationHeader === undefined) throw new FiscalError("VALIDATION_ERROR");
      const identity = await authorizer.authorize(
        header(request, "authorization"),
        "fiscal",
        organizationId,
        correlationHeader,
      );
      if (identity.correlationId !== correlationHeader) throw new FiscalError("UNAUTHORIZED_SERVICE");
      const context = { idempotencyKey, correlationId: correlationHeader, identity };
      const fiscalRequest = await readFiscalRequest(
        request,
        requestId !== undefined,
      );
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
