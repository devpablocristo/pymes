import assert from "node:assert/strict";
import test from "node:test";
import type {
  ArtifactRepository,
  EnvelopeCipher,
  SealedValue,
  StoredAccessTicket,
  StoredArtifact,
  TicketRepository,
} from "../../src/credentials/usecases.js";
import {
  ArcaFiscalAuthority,
  type ArcaClientFactory,
} from "../../src/fiscal/arca.js";
import { compatibleExplicitClient } from "../../src/fiscal/arca/helpers/client.js";
import { mapFiscalRequest } from "../../src/fiscal/arca/helpers/mapping.js";
import type {
  ExplicitSDKClient,
  SDKAuthorizationResponse,
  SDKConsultResponse,
  SDKInvoiceRequest,
} from "../../src/fiscal/arca/models/sdk.js";
import {
  FiscalError,
  type FiscalRequest,
} from "../../src/fiscal/usecases/domain/fiscal.js";

test("real mapping keeps explicit numbering, deterministic VAT and associated note", () => {
  const request = fiscalRequest();
  request.document_type = "NCA";
  request.associated_voucher = {
    point_of_sale: 4,
    document_type: "FA",
    voucher_number: 40,
    issue_date: "2026-08-01",
  };
  const mapped = mapFiscalRequest(request);

  assert.equal(mapped.PtoVta, 4);
  assert.equal(mapped.CbteTipo, 3);
  assert.equal(mapped.invoices[0]?.CbteDesde, 41);
  assert.equal(mapped.invoices[0]?.CbteHasta, 41);
  assert.deepEqual(mapped.invoices[0]?.Iva, [
    { Id: 5, BaseImp: 100, Importe: 21 },
  ]);
  assert.deepEqual(mapped.invoices[0]?.CbtesAsoc, [
    { Tipo: 1, PtoVta: 4, Nro: 40, CbteFch: "20260801" },
  ]);
});

test("real mapping supports services, foreign currency and type C without VAT discrimination", () => {
  const service = fiscalRequest();
  service.document_type = "FB";
  service.concept = "services";
  service.service_period = {
    from: "2026-08-01",
    to: "2026-08-31",
    payment_due: "2026-09-10",
  };
  service.currency = "USD";
  service.exchange_rate = "1310.123456";
  const mappedService = mapFiscalRequest(service).invoices[0]!;
  assert.equal(mappedService.Concepto, 2);
  assert.equal(mappedService.MonId, "DOL");
  assert.equal(mappedService.MonCotiz, 1310.123456);
  assert.equal(mappedService.FchServDesde, "20260801");
  assert.equal(mappedService.FchServHasta, "20260831");
  assert.equal(mappedService.FchVtoPago, "20260910");

  const typeC = fiscalRequest();
  typeC.document_type = "FC";
  typeC.totals = { net: "121.00", vat: "0", exempt: "0", total: "121.00" };
  typeC.lines = [{
    description: "Servicio monotributo",
    quantity: "1",
    unit_price: "121",
    vat_rate: "0",
    net: "121",
  }];
  const mappedC = mapFiscalRequest(typeC).invoices[0]!;
  assert.equal(mappedC.Iva, undefined);

  const invalid = fiscalRequest();
  invalid.totals.vat = "20.99";
  assert.throws(
    () => mapFiscalRequest(invalid),
    (error: unknown) =>
      error instanceof FiscalError && error.code === "VALIDATION_ERROR",
  );
});

test("real authority consults exact voucher before issuing and stores encrypted result", async () => {
  const artifacts = new MemoryArtifacts();
  const client = new ScriptedClient(
    [{}],
    [{
      FeDetResp: {
        FECAEDetResponse: {
          Resultado: "A",
          CAE: "71234567890123",
          CAEFchVto: "20260815",
        },
      },
    }],
  );
  const authority = authorityFor(client, artifacts);
  const request = fiscalRequest();
  const result = await authority.authorize(request);

  assert.equal(result.status, "authorized");
  assert.equal(result.cae, "71234567890123");
  assert.match(result.artifact_ref ?? "", /^fartifact_/);
  assert.deepEqual(client.consulted, [{
    pointOfSale: 4,
    voucherType: 1,
    voucherNumber: 41,
  }]);
  assert.equal(client.authorized.length, 1);
  assert.equal(client.authorized[0]?.invoices[0]?.CbteDesde, 41);
  assert.equal(artifacts.values.length, 1);
  assert.equal(artifacts.values[0]?.organizationId, "org_real");
});

test("an existing exact voucher is reconciled and never authorized twice", async () => {
  const request = fiscalRequest();
  const client = new ScriptedClient(
    [consultedVoucher(request)],
    [],
  );
  const authority = authorityFor(client, new MemoryArtifacts());
  const result = await authority.authorize(request);

  assert.equal(result.status, "authorized");
  assert.equal(result.cae, "71234567890123");
  assert.equal(client.authorized.length, 0);
});

test("response loss after dispatch is uncertain while pre-dispatch failure is retryable", async () => {
  const dispatchedClient = new ScriptedClient([{}], [new Error("socket lost")]);
  const dispatched = authorityFor(
    dispatchedClient,
    new MemoryArtifacts(),
    true,
  );
  assert.equal((await dispatched.authorize(fiscalRequest())).status, "uncertain");

  const beforeClient = new ScriptedClient([{}], [new Error("DNS unavailable")]);
  const before = authorityFor(beforeClient, new MemoryArtifacts(), false);
  await assert.rejects(
    before.authorize(fiscalRequest()),
    (error: unknown) =>
      error instanceof FiscalError && error.code === "AUTHORITY_TIMEOUT",
  );
});

test("consultation rejects an occupied number whose ARCA snapshot differs", async () => {
  const request = fiscalRequest();
  const existing = consultedVoucher(request);
  existing.ResultGet!.ImpTotal = 999;
  const result = await authorityFor(
    new ScriptedClient([existing], []),
    new MemoryArtifacts(),
  ).authorize(request);

  assert.equal(result.status, "rejected");
  assert.equal(result.result_code, "VOUCHER_MISMATCH");
});

test("point-of-sale validation uses FEParamGetPtosVenta without a synthetic voucher", async () => {
  const client = new ScriptedClient([], [], [[{
    number: 9,
    emissionType: "CAE",
    blocked: false,
  }]]);
  const authority = authorityFor(client, new MemoryArtifacts());
  await authority.validatePointOfSale({
    material: credentialMaterial(),
    pointOfSale: 9,
  });
  assert.equal(client.pointsOfSaleListed, 1);
  assert.deepEqual(client.consulted, []);
  assert.equal(client.authorized.length, 0);

  const providerError = new Error("invalid point of sale");
  providerError.name = "ArcaWSFEError";
  const invalid = authorityFor(
    new ScriptedClient([], [], [providerError]),
    new MemoryArtifacts(),
  );
  await assert.rejects(
    invalid.validatePointOfSale({
      material: credentialMaterial(),
      pointOfSale: 10,
    }),
    (error: unknown) =>
      error instanceof Error &&
      "code" in error &&
      (error as { code: unknown }).code ===
        "POINT_OF_SALE_NOT_VALIDATED",
  );

  const unavailable = authorityFor(
    new ScriptedClient([], [], [new Error("network unavailable")]),
    new MemoryArtifacts(),
  );
  await assert.rejects(
    unavailable.validatePointOfSale({
      material: credentialMaterial(),
      pointOfSale: 11,
    }),
    (error: unknown) =>
      error instanceof FiscalError && error.code === "AUTHORITY_TIMEOUT",
  );
});

test("point-of-sale validation rejects missing, blocked and malformed entries", async () => {
  for (const listed of [
    [],
    [{
      number: 12,
      emissionType: "CAE",
      blocked: true,
    }],
    [{
      number: 12,
      emissionType: "CAE",
      blocked: false,
      deactivatedOn: "20261231",
    }],
  ]) {
    const authority = authorityFor(
      new ScriptedClient([], [], [listed]),
      new MemoryArtifacts(),
    );
    await assert.rejects(
      authority.validatePointOfSale({
        material: credentialMaterial(),
        pointOfSale: 12,
      }),
      (error: unknown) =>
        error instanceof Error &&
        "code" in error &&
        (error as { code: unknown }).code ===
          "POINT_OF_SALE_NOT_VALIDATED",
    );
  }

  const malformed = new Error("invalid provider payload");
  malformed.name = "ExplicitPointOfSaleError";
  const authority = authorityFor(
    new ScriptedClient([], [], [malformed]),
    new MemoryArtifacts(),
  );
  await assert.rejects(
    authority.validatePointOfSale({
      material: credentialMaterial(),
      pointOfSale: 12,
    }),
    (error: unknown) =>
      error instanceof FiscalError && error.code === "INTERNAL_ERROR",
  );
});

test("published SDK compatibility uses FEParamGetPtosVenta until the explicit method is released", async () => {
  let consulted = 0;
  let legacyListed = 0;
  const compatible = compatibleExplicitClient(
    {
      async authorize() {
        return {};
      },
      async consult() {
        consulted += 1;
        return {};
      },
    },
    {
      async getPuntosVenta() {
        legacyListed += 1;
        return [{
          Nro: 15,
          EmisionTipo: "CAE",
          Bloqueado: "N",
          FchBaja: "NULL",
        }];
      },
    },
  );

  assert.deepEqual(await compatible.listPointsOfSale(), [{
    number: 15,
    emissionType: "CAE",
    blocked: false,
  }]);
  assert.equal(legacyListed, 1);
  assert.equal(consulted, 0);
});

test("published SDK compatibility prefers the explicit point-of-sale API", async () => {
  let explicitListed = 0;
  const futureExplicitClient = {
    async authorize() {
      return {};
    },
    async consult() {
      return {};
    },
    async listPointsOfSale() {
      explicitListed += 1;
      return [{
        number: 16,
        emissionType: "CAE",
        blocked: false,
      }];
    },
  };
  const compatible = compatibleExplicitClient(
    futureExplicitClient,
    {
      async getPuntosVenta() {
        throw new Error("legacy point-of-sale API must not be called");
      },
    },
  );

  assert.deepEqual(await compatible.listPointsOfSale(), [{
    number: 16,
    emissionType: "CAE",
    blocked: false,
  }]);
  assert.equal(explicitListed, 1);
});

function authorityFor(
  client: ScriptedClient,
  artifacts: ArtifactRepository,
  emitDispatch = true,
): ArcaFiscalAuthority {
  const factory: ArcaClientFactory = {
    create(input) {
      client.onEvent = emitDispatch ? input.onEvent : undefined;
      return client;
    },
  };
  return new ArcaFiscalAuthority(
    {
      async resolveMaterial() {
        return credentialMaterial();
      },
    },
    new MemoryTickets(),
    artifacts,
    new PassthroughCipher(),
    { clientFactory: factory, requestTimeoutMs: 2_000 },
  );
}

class ScriptedClient implements ExplicitSDKClient {
  readonly consulted: Array<{
    pointOfSale: number;
    voucherType: number;
    voucherNumber: number;
  }> = [];
  readonly authorized: SDKInvoiceRequest[] = [];
  pointsOfSaleListed = 0;
  onEvent?: (event: { type: string; method?: string }) => void;

  constructor(
    private readonly consultResults: Array<SDKConsultResponse | Error>,
    private readonly authorizeResults: Array<SDKAuthorizationResponse | Error>,
    private readonly pointOfSaleResults: Array<
      Awaited<ReturnType<ExplicitSDKClient["listPointsOfSale"]>> | Error
    > = [],
  ) {}

  async consult(reference: {
    pointOfSale: number;
    voucherType: number;
    voucherNumber: number;
  }): Promise<SDKConsultResponse> {
    this.consulted.push(reference);
    const result = this.consultResults.shift();
    if (result instanceof Error) throw result;
    return result ?? {};
  }

  async authorize(
    request: SDKInvoiceRequest,
  ): Promise<SDKAuthorizationResponse> {
    this.authorized.push(request);
    this.onEvent?.({ type: "request:start", method: "FECAESolicitar" });
    const result = this.authorizeResults.shift();
    if (result instanceof Error) throw result;
    return result ?? {};
  }

  async listPointsOfSale() {
    this.pointsOfSaleListed += 1;
    const result = this.pointOfSaleResults.shift();
    if (result instanceof Error) throw result;
    return result ?? [];
  }
}

class PassthroughCipher implements EnvelopeCipher {
  async seal(plaintext: Uint8Array): Promise<SealedValue> {
    return {
      format: "aes-256-gcm+kms-v1",
      ciphertext: Buffer.from(plaintext).toString("base64"),
      encryptedDataKey: Buffer.alloc(32, 1).toString("base64"),
      iv: Buffer.alloc(12, 2).toString("base64"),
      authTag: Buffer.alloc(16, 3).toString("base64"),
      kmsKeyName: "test",
    };
  }

  async open(value: SealedValue): Promise<Uint8Array> {
    return Buffer.from(value.ciphertext, "base64");
  }
}

class MemoryTickets implements TicketRepository {
  async findTicket(): Promise<StoredAccessTicket | undefined> {
    return undefined;
  }
  async saveTicket(): Promise<void> {}
  async deleteTicket(): Promise<void> {}
}

class MemoryArtifacts implements ArtifactRepository {
  readonly values: StoredArtifact[] = [];
  async saveArtifact(artifact: StoredArtifact): Promise<void> {
    this.values.push(artifact);
  }
}

function fiscalRequest(): FiscalRequest {
  return {
    request_id: "fiscal:sale-41:1",
    organization_id: "org_real",
    idempotency_key: "fiscal:sale-41:1",
    correlation_id: "sale-41",
    source_version: 1,
    credential_ref: "fcred_00000001",
    environment: "homologation",
    point_of_sale: 4,
    document_type: "FA",
    voucher_number: 41,
    issue_date: "2026-08-01",
    concept: "products",
    currency: "ARS",
    totals: { net: "100.00", vat: "21.00", exempt: "0", total: "121.00" },
    recipient: {
      document_type: "CUIT",
      document_number: "30710158202",
      vat_condition: "RESPONSABLE_INSCRIPTO",
    },
    lines: [{
      description: "Servicio",
      quantity: "1",
      unit_price: "100",
      vat_rate: "21",
      net: "100.00",
    }],
    snapshot_digest: "d".repeat(64),
  };
}

function credentialMaterial() {
  return {
    credential: {
      id: "fcred_00000001",
      organizationId: "org_real",
      cuit: "20123456786",
      environment: "homologation" as const,
      legalName: "Cliente Real SA",
      commonName: "cliente-real",
      status: "ready" as const,
      certificateFingerprint: "a".repeat(64),
      certificateValidFrom: "2026-01-01T00:00:00.000Z",
      certificateExpiresAt: "2027-01-01T00:00:00.000Z",
      certificateSerialNumber: "01",
      version: 2,
      createdAt: "2026-01-01T00:00:00.000Z",
      updatedAt: "2026-01-01T00:00:00.000Z",
    },
    certificatePem: "certificate",
    privateKeyPem: "private-key",
  };
}

function consultedVoucher(request: FiscalRequest): SDKConsultResponse {
  return {
    ResultGet: {
      PtoVta: request.point_of_sale,
      CbteTipo: 1,
      CbteDesde: request.voucher_number,
      CbteHasta: request.voucher_number,
      DocTipo: 80,
      DocNro: Number(request.recipient.document_number),
      ImpTotal: 121,
      ImpNeto: 100,
      ImpOpEx: 0,
      ImpIVA: 21,
      MonId: "PES",
      MonCotiz: 1,
      Resultado: "A",
      CodAutorizacion: "71234567890123",
      FchVto: "20260815",
    },
  };
}
