import {
  createExplicitArcaClient,
  createWSAAAccessTicketProvider,
} from "@devpablocristo/arca-facturacion/explicit";
import type {
  ArtifactRepository,
  CredentialProbe,
  CredentialProbeInput,
  CredentialMaterial,
  EnvelopeCipher,
  TicketRepository,
} from "../credentials/usecases.js";
import { CredentialError } from "../credentials/usecases/domain/credential.js";
import type { AuthorityDecision, FiscalAuthority } from "./usecases.js";
import type { FiscalRequest } from "./usecases/domain/fiscal.js";
import { FiscalError } from "./usecases/domain/fiscal.js";
import type { ExplicitSDKClient } from "./arca/models/sdk.js";
import {
  mapFiscalRequest,
  supportedVoucherTypes,
  voucherType,
} from "./arca/helpers/mapping.js";
import {
  authorizationDecision,
  authorizationFailure,
  consultationDecision,
  consultationFailure,
} from "./arca/helpers/responses.js";
import { DurableWSAATicketProvider } from "./arca/helpers/tickets.js";
import {
  artifactAAD,
  artifactReference,
} from "./arca/helpers/artifacts.js";
import { validatedExplicitClient } from "./arca/helpers/client.js";
import { providerErrorName } from "./arca/helpers/errors.js";

export interface CredentialMaterialResolver {
  resolveMaterial(input: {
    organizationId: string;
    credentialId: string;
    environment: FiscalRequest["environment"];
    pointOfSale: number;
  }): Promise<CredentialMaterial>;
}

export interface ArcaAuthorityOptions {
  requestTimeoutMs?: number;
  onOperationalEvent?: (event: {
    type: string;
    method?: string;
    durationMs?: number;
    attempt?: number;
  }) => void;
  clientFactory?: ArcaClientFactory;
}

export interface ArcaClientFactory {
  create(input: {
    material: CredentialMaterial;
    tickets: TicketRepository;
    cipher: EnvelopeCipher;
    requestTimeoutMs: number;
    onEvent?: (event: {
      type: string;
      method?: string;
      durationMs?: number;
      attempt?: number;
    }) => void;
  }): ExplicitSDKClient;
}

export class ArcaFiscalAuthority
  implements FiscalAuthority, CredentialProbe
{
  constructor(
    private readonly credentials: CredentialMaterialResolver,
    private readonly tickets: TicketRepository,
    private readonly artifacts: ArtifactRepository,
    private readonly cipher: EnvelopeCipher,
    private readonly options: ArcaAuthorityOptions = {},
  ) {}

  async authorize(request: FiscalRequest): Promise<AuthorityDecision> {
    const material = await this.resolveMaterial(request);
    let authorizationDispatched = false;
    const client = this.client(material, (event) => {
      if (
        event.type === "request:start" &&
        event.method === "FECAESolicitar"
      ) {
        authorizationDispatched = true;
      }
      this.options.onOperationalEvent?.(event);
    });

    const existing = await this.consultWithClient(client, request);
    if (existing.status !== "not_found") return existing;

    try {
      const response = await client.authorize(mapFiscalRequest(request));
      return await this.withArtifact(
        request,
        authorizationDecision(response),
        response,
        "wsfe_authorization",
      );
    } catch (error) {
      return authorizationFailure(error, authorizationDispatched);
    }
  }

  async consult(request: FiscalRequest): Promise<AuthorityDecision> {
    const material = await this.resolveMaterial(request);
    return this.consultWithClient(this.client(material), request);
  }

  async validatePointOfSale(
    input: CredentialProbeInput,
  ): Promise<void> {
    try {
      const client = this.client(input.material);
      const pointsOfSale = await client.listPointsOfSale();
      const configured = pointsOfSale.find(
        (point) => point.number === input.pointOfSale,
      );
      if (
        configured === undefined ||
        configured.blocked ||
        configured.deactivatedOn !== undefined
      ) {
        throw new CredentialError("POINT_OF_SALE_NOT_VALIDATED");
      }
      for (const sequenceVoucherType of supportedVoucherTypes) {
        const baseline = await client.lastAuthorizedVoucher({
          pointOfSale: input.pointOfSale,
          voucherType: sequenceVoucherType,
        });
        if (baseline.voucherNumber !== 0) {
          throw new CredentialError("POINT_OF_SALE_NOT_EMPTY");
        }
      }
    } catch (error) {
      if (error instanceof CredentialError) throw error;
      if (providerErrorName(error) === "ExplicitPointOfSaleError") {
        throw new FiscalError(
          "INTERNAL_ERROR",
          "ARCA returned an invalid point-of-sale response",
        );
      }
      if (
        providerErrorName(error) === "ArcaWSFEError"
      ) {
        throw new CredentialError("POINT_OF_SALE_NOT_VALIDATED");
      }
      throw new FiscalError("AUTHORITY_TIMEOUT");
    }
  }

  private async consultWithClient(
    client: ExplicitSDKClient,
    request: FiscalRequest,
  ): Promise<AuthorityDecision> {
    try {
      const response = await client.consult({
        pointOfSale: request.point_of_sale,
        voucherType: voucherType(request),
        voucherNumber: request.voucher_number,
      });
      return await this.withArtifact(
        request,
        consultationDecision(request, response),
        response,
        "wsfe_consultation",
      );
    } catch (error) {
      return consultationFailure(error);
    }
  }

  private client(
    material: CredentialMaterial,
    onEvent?: (event: {
      type: string;
      method?: string;
      durationMs?: number;
      attempt?: number;
    }) => void,
  ): ExplicitSDKClient {
    const requestTimeoutMs = this.options.requestTimeoutMs ?? 30_000;
    if (this.options.clientFactory !== undefined) {
      return this.options.clientFactory.create({
        material,
        tickets: this.tickets,
        cipher: this.cipher,
        requestTimeoutMs,
        ...(onEvent === undefined ? {} : { onEvent }),
      });
    }
    const baseProvider = createWSAAAccessTicketProvider({
      cert: material.certificatePem,
      key: material.privateKeyPem,
      production: material.credential.environment === "production",
      requestTimeoutMs,
      retries: 1,
      retryDelayMs: 1_000,
      onEvent,
    });
    const accessTicketProvider = new DurableWSAATicketProvider(
      baseProvider,
      this.tickets,
      this.cipher,
      {
        organizationId: material.credential.organizationId,
        credentialId: material.credential.id,
        environment: material.credential.environment,
      },
    );
    const config = {
      cuit: Number(material.credential.cuit),
      cert: material.certificatePem,
      key: material.privateKeyPem,
      production: material.credential.environment === "production",
      accessTicketProvider,
      requestTimeoutMs,
      retries: 0,
      retryDelayMs: 1_000,
      onEvent,
    };
    return validatedExplicitClient(createExplicitArcaClient(config));
  }

  private async resolveMaterial(
    request: FiscalRequest,
  ): Promise<CredentialMaterial> {
    try {
      return await this.credentials.resolveMaterial({
        organizationId: request.organization_id,
        credentialId: request.credential_ref,
        environment: request.environment,
        pointOfSale: request.point_of_sale,
      });
    } catch (error) {
      if (error instanceof CredentialError) {
        throw new FiscalError("VALIDATION_ERROR", error.code);
      }
      throw error;
    }
  }

  private async withArtifact(
    request: FiscalRequest,
    decision: AuthorityDecision,
    providerPayload: unknown,
    kind: "wsfe_authorization" | "wsfe_consultation",
  ): Promise<AuthorityDecision> {
    if (decision.status !== "authorized") return decision;
    const artifactId = artifactReference(request, kind);
    try {
      const encryptedPayload = await this.cipher.seal(
        Buffer.from(JSON.stringify(providerPayload), "utf8"),
        artifactAAD(request, artifactId, kind),
      );
      await this.artifacts.saveArtifact({
        organizationId: request.organization_id,
        artifactId,
        requestId: request.request_id,
        kind,
        encryptedPayload,
      });
      return { ...decision, artifact_ref: artifactId };
    } catch {
      // El CAE conocido prevalece sobre el artefacto operacional. Un timeout
      // posterior se reconcilia por consulta exacta y nunca por reemisión.
      return decision;
    }
  }
}
