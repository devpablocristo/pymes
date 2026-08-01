import type { IncomingMessage, ServerResponse } from "node:http";
import type { InternalAuthorizer } from "../fiscal/handler.js";
import type {
  ConfigurePointOfSaleInput,
  CSRResult,
  CredentialActor,
  RequestCSRInput,
  UploadCertificateInput,
} from "./usecases.js";
import type {
  CredentialMetadata,
  PointOfSale,
} from "./usecases/domain/credential.js";
import type {
  CertificateUploadDTO,
  CSRRequestDTO,
  PointOfSaleDTO,
} from "./handler/models/http.js";
import {
  header,
  readJSON,
  respond,
  respondProblem,
  credentialDTO,
  csrResultDTO,
  pointOfSaleDTO,
} from "./handler/helpers/http.js";
import { CredentialError } from "./usecases/domain/credential.js";

export interface CredentialApplication {
  requestCSR(
    input: RequestCSRInput,
    actor: CredentialActor,
  ): Promise<CSRResult>;
  uploadCertificate(
    input: UploadCertificateInput,
    actor: CredentialActor,
  ): Promise<CredentialMetadata>;
  configurePointOfSale(
    input: ConfigurePointOfSaleInput,
    actor: CredentialActor,
  ): Promise<PointOfSale>;
  validatePointOfSale(
    input: ConfigurePointOfSaleInput,
    actor: CredentialActor,
  ): Promise<PointOfSale>;
  getCredential(
    organizationId: string,
    credentialId: string,
    actor: CredentialActor,
  ): Promise<CredentialMetadata>;
}

export async function handleCredentialRequest(
  request: IncomingMessage,
  response: ServerResponse,
  application: CredentialApplication,
  authorizer: InternalAuthorizer,
): Promise<boolean> {
  const url = request.url ?? "";
  const csrMatch = url.match(
    /^\/internal\/v1\/organizations\/([^/]+)\/credentials\/csr$/,
  );
  const credentialMatch = url.match(
    /^\/internal\/v1\/organizations\/([^/]+)\/credentials\/([^/]+)$/,
  );
  const pointOfSaleMatch = url.match(
    /^\/internal\/v1\/organizations\/([^/]+)\/credentials\/([^/]+)\/points-of-sale\/(\d+)$/,
  );
  const pointOfSaleValidationMatch = url.match(
    /^\/internal\/v1\/organizations\/([^/]+)\/credentials\/([^/]+)\/points-of-sale\/(\d+)\/validate$/,
  );
  if (
    csrMatch === null &&
    credentialMatch === null &&
    pointOfSaleMatch === null &&
    pointOfSaleValidationMatch === null
  ) {
    return false;
  }

  const correlationId =
    header(request, "x-correlation-id") ?? "missing-correlation-id";
  try {
    const rawOrganizationId =
      csrMatch?.[1] ??
      credentialMatch?.[1] ??
      pointOfSaleMatch?.[1] ??
      pointOfSaleValidationMatch?.[1];
    if (rawOrganizationId === undefined) {
      throw new CredentialError("VALIDATION_ERROR");
    }
    const organizationId = decodeURIComponent(rawOrganizationId);
    const identity = await authorizer.authorize(
      header(request, "authorization"),
      "fiscal",
      organizationId,
      correlationId,
    );
    const actor: CredentialActor = {
      organizationId: identity.organizationId,
      subject: identity.subject,
      roles: identity.roles,
      correlationId: identity.correlationId,
    };

    if (csrMatch !== null && request.method === "POST") {
      const idempotencyKey = header(request, "idempotency-key");
      if (idempotencyKey === undefined) {
        throw new CredentialError("VALIDATION_ERROR");
      }
      const body = await readJSON<CSRRequestDTO>(request);
      const result = await application.requestCSR(
        {
          organizationId,
          cuit: body.cuit,
          environment: body.environment,
          legalName: body.legal_name,
          commonName: body.common_name,
          idempotencyKey,
        },
        actor,
      );
      respond(response, 201, csrResultDTO(result));
      return true;
    }

    if (credentialMatch !== null) {
      const credentialId = decodeURIComponent(credentialMatch[2]!);
      if (request.method === "GET") {
        respond(
          response,
          200,
          credentialDTO(
            await application.getCredential(
              organizationId,
              credentialId,
              actor,
            ),
          ),
        );
        return true;
      }
      if (request.method === "PUT") {
        const body = await readJSON<CertificateUploadDTO>(request);
        respond(
          response,
          200,
          credentialDTO(
            await application.uploadCertificate(
              {
                organizationId,
                credentialId,
                certificatePem: body.certificate_pem,
                expectedVersion: body.expected_version,
              },
              actor,
            ),
          ),
        );
        return true;
      }
    }

    if (pointOfSaleMatch !== null && request.method === "PUT") {
      const body = await readJSON<PointOfSaleDTO>(request);
      respond(
        response,
        200,
        pointOfSaleDTO(
          await application.configurePointOfSale(
            {
              organizationId,
              credentialId: decodeURIComponent(pointOfSaleMatch[2]!),
              number: Number(pointOfSaleMatch[3]),
              enabled: body.enabled,
            },
            actor,
          ),
        ),
      );
      return true;
    }

    if (
      pointOfSaleValidationMatch !== null &&
      request.method === "POST"
    ) {
      const body = await readJSON<PointOfSaleDTO>(request);
      respond(
        response,
        200,
        pointOfSaleDTO(
          await application.validatePointOfSale(
            {
              organizationId,
              credentialId: decodeURIComponent(
                pointOfSaleValidationMatch[2]!,
              ),
              number: Number(pointOfSaleValidationMatch[3]),
              enabled: body.enabled,
            },
            actor,
          ),
        ),
      );
      return true;
    }

    respond(response, 405, { code: "METHOD_NOT_ALLOWED" });
    return true;
  } catch (error) {
    respondProblem(response, error, correlationId);
    return true;
  }
}
