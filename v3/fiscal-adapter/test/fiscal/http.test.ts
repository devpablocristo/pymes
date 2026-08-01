import assert from "node:assert/strict";
import type { AddressInfo } from "node:net";
import test from "node:test";
import { InMemoryFiscalLedger } from "../../src/fiscal/in_memory_ledger.js";
import { MockFiscalAuthority } from "../../src/fiscal/mock_authority.js";
import {
  FiscalError,
  type FiscalProblem,
  type FiscalRequest,
  type FiscalResult,
} from "../../src/fiscal/usecases/domain/fiscal.js";
import {
  createFiscalHTTPServer,
  type FiscalApplication,
  type FiscalRuntimeObserver,
  type InternalAuthorizer,
} from "../../src/fiscal/handler.js";
import type { InternalIdentity } from "../../src/fiscal/usecases.js";
import { FiscalService } from "../../src/fiscal/usecases.js";
import { InsecureLocalAuthorizer } from "../../src/identity/insecure_local.js";
import type { CredentialApplication } from "../../src/credentials/handler.js";
import { createFiscalRuntimeObserver } from "../../src/wire.js";

const request: FiscalRequest = {
  request_id: "fiscal:sale-http:1",
  organization_id: "org_http",
  idempotency_key: "fiscal:sale-http:1",
  correlation_id: "http-status-test",
  source_version: 1,
  credential_ref: "mock://credential/http",
  environment: "homologation",
  point_of_sale: 3,
  document_type: "FB",
  voucher_number: 8,
  issue_date: "2026-07-30",
  concept: "products",
  currency: "ARS",
  totals: { net: "100", vat: "21", exempt: "0", total: "121" },
  recipient: { document_type: "CUIT", document_number: "20123456789", vat_condition: "registered" },
  lines: [{ description: "Servicio", quantity: "1", unit_price: "100", vat_rate: "21", net: "100" }],
  snapshot_digest: "b".repeat(64),
};

const healthyRuntime: FiscalRuntimeObserver = {
  async ping() {},
  async metrics() {
    return { authorized: 1, rejected: 2, uncertain: 3, not_found: 4 };
  },
};

test("private HTTP contract exposes uncertain and exact reconciliation", async (t) => {
  const service = new FiscalService(new MockFiscalAuthority("response_lost_after_processing"), new InMemoryFiscalLedger(), () => new Date("2026-07-30T12:00:00.000Z"));
  const server = createFiscalHTTPServer(service, new InsecureLocalAuthorizer(), healthyRuntime);
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(() => new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve())));
  const { port } = server.address() as AddressInfo;
  const base = `http://127.0.0.1:${port}/internal/v1/organizations/${request.organization_id}/authorizations`;
  const headers = { "content-type": "application/json", "idempotency-key": request.idempotency_key, "x-correlation-id": request.correlation_id };

  const authorize = await fetch(base, { method: "POST", headers, body: JSON.stringify(request) });
  assert.equal(authorize.status, 202);
  assert.equal(authorize.headers.get("cache-control"), "no-store");
  assert.equal(((await authorize.json()) as FiscalResult).status, "uncertain");

  const consult = await fetch(`${base}/${encodeURIComponent(request.request_id)}/consult`, { method: "POST", headers, body: JSON.stringify(request) });
  assert.equal(consult.status, 200);
  const result = (await consult.json()) as FiscalResult;
  assert.equal(result.status, "authorized");
  assert.equal(result.idempotency_key, request.idempotency_key);
  assert.equal(result.source_version, request.source_version);
  assert.equal(result.snapshot_digest, request.snapshot_digest);
  assert.equal(result.correlation_id, request.correlation_id);
  assert.equal(result.cae?.length, 14);
});

test("readiness and metrics reflect the durable runtime", async () => {
  await withHTTP(new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger()), new InsecureLocalAuthorizer(), async (origin) => {
    assert.equal((await fetch(`${origin}/readyz`)).status, 200);
    const metrics = await fetch(`${origin}/metrics`);
    assert.equal(metrics.status, 200);
    assert.match(await metrics.text(), /pymes_fiscal_results\{status="uncertain"\} 3/);
  });
  const unavailable: FiscalRuntimeObserver = {
    async ping() { throw new Error("database unavailable"); },
    async metrics() { throw new Error("database unavailable"); },
  };
  const server = createFiscalHTTPServer(
    new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger()),
    new InsecureLocalAuthorizer(),
    unavailable,
  );
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  try {
    const { port } = server.address() as AddressInfo;
    assert.equal((await fetch(`http://127.0.0.1:${port}/readyz`)).status, 503);
    assert.equal((await fetch(`http://127.0.0.1:${port}/metrics`)).status, 503);
  } finally {
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
});

test("readiness fails and recovers with the fiscal KMS runtime", async () => {
  let kmsAvailable = false;
  const runtime = createFiscalRuntimeObserver(healthyRuntime, {
    async ready() {
      if (!kmsAvailable) throw new Error("KMS unavailable");
    },
  });
  const server = createFiscalHTTPServer(
    new FiscalService(
      new MockFiscalAuthority(),
      new InMemoryFiscalLedger(),
    ),
    new InsecureLocalAuthorizer(),
    runtime,
  );
  await new Promise<void>((resolve) =>
    server.listen(0, "127.0.0.1", resolve)
  );
  try {
    const { port } = server.address() as AddressInfo;
    const readiness = `http://127.0.0.1:${port}/readyz`;
    assert.equal((await fetch(readiness)).status, 503);
    kmsAvailable = true;
    assert.equal((await fetch(readiness)).status, 200);
  } finally {
    await new Promise<void>((resolve, reject) =>
      server.close((error) => error ? reject(error) : resolve())
    );
  }
});

test("catalog is implemented and private routes reject invalid workload identity", async () => {
  await withHTTP(new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger()), new InsecureLocalAuthorizer(), async (origin) => {
    const catalog = await fetch(`${origin}/internal/v1/catalogs/document-types`, { headers: { "x-correlation-id": "catalog-test" } });
    assert.equal(catalog.status, 200);
    const values = await catalog.json() as Array<{ code: string; letter: string; kind: string }>;
    assert.equal(values.length, 9);
    assert.deepEqual(values[0], { code: "FA", letter: "A", kind: "invoice" });
  });

  const denied: InternalAuthorizer = {
    async authorize() {
      throw new FiscalError("UNAUTHORIZED_SERVICE");
    },
  };
  await withHTTP(new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger()), denied, async (origin) => {
    const catalog = await fetch(`${origin}/internal/v1/catalogs/document-types`, { headers: { "x-correlation-id": "catalog-test" } });
    assert.equal(catalog.status, 401);
    const authorize = await fetch(`${origin}/internal/v1/organizations/${request.organization_id}/authorizations`, {
      method: "POST",
      headers: requestHeaders,
      body: JSON.stringify(request),
    });
    assert.equal(authorize.status, 401);
  });
});

test("credential onboarding returns only snake-case metadata and the public CSR", async () => {
  const credential = {
    id: "fcred_00000001",
    organizationId: "org_http",
    cuit: "20123456786",
    environment: "homologation" as const,
    legalName: "Cliente HTTP SA",
    commonName: "cliente-http",
    status: "pending_certificate" as const,
    version: 1,
    createdAt: "2026-08-01T12:00:00.000Z",
    updatedAt: "2026-08-01T12:00:00.000Z",
  };
  const credentials: CredentialApplication = {
    async requestCSR() {
      return { credential, csrPem: "-----BEGIN CERTIFICATE REQUEST-----" };
    },
    async uploadCertificate() {
      return { ...credential, status: "ready", version: 2 };
    },
    async configurePointOfSale() {
      return {
        organizationId: credential.organizationId,
        credentialId: credential.id,
        environment: credential.environment,
        number: 4,
        enabled: true,
      };
    },
    async validatePointOfSale() {
      return {
        organizationId: credential.organizationId,
        credentialId: credential.id,
        environment: credential.environment,
        number: 4,
        enabled: true,
        validatedAt: "2026-08-01T12:30:00.000Z",
      };
    },
    async getCredential() {
      return credential;
    },
  };
  await withHTTP(
    new FiscalService(
      new MockFiscalAuthority(),
      new InMemoryFiscalLedger(),
    ),
    new InsecureLocalAuthorizer(),
    async (origin) => {
      const response = await fetch(
        `${origin}/internal/v1/organizations/org_http/credentials/csr`,
        {
          method: "POST",
          headers: {
            "content-type": "application/json",
            "idempotency-key": "credential:http:1",
            "x-correlation-id": "credential:http:1",
          },
          body: JSON.stringify({
            cuit: credential.cuit,
            environment: credential.environment,
            legal_name: credential.legalName,
            common_name: credential.commonName,
          }),
        },
      );
      assert.equal(response.status, 201);
      const body = (await response.json()) as Record<string, unknown>;
      assert.equal(body.csr_pem, "-----BEGIN CERTIFICATE REQUEST-----");
      assert.equal("csrPem" in body, false);
      const metadata = body.credential as Record<string, unknown>;
      assert.equal(metadata.organization_id, "org_http");
      assert.equal(metadata.legal_name, "Cliente HTTP SA");
      assert.equal("organizationId" in metadata, false);
      assert.equal(JSON.stringify(body).includes("privateKeyPem"), false);
      assert.equal(
        JSON.stringify(body).includes("encryptedPrivateKey"),
        false,
      );

      const validation = await fetch(
        `${origin}/internal/v1/organizations/org_http/credentials/fcred_00000001/points-of-sale/4/validate`,
        {
          method: "POST",
          headers: {
            "content-type": "application/json",
            "x-correlation-id": "credential:http:validate",
          },
          body: JSON.stringify({ enabled: true }),
        },
      );
      assert.equal(validation.status, 200);
      const pointOfSale = (await validation.json()) as Record<
        string,
        unknown
      >;
      assert.equal(pointOfSale.organization_id, "org_http");
      assert.equal(
        pointOfSale.validated_at,
        "2026-08-01T12:30:00.000Z",
      );
    },
    credentials,
  );
});

test("HTTP statuses distinguish rejected, not found, conflict, validation and timeout", async () => {
  await withHTTP(new FiscalService(new MockFiscalAuthority("rejected"), new InMemoryFiscalLedger()), new InsecureLocalAuthorizer(), async (origin) => {
    const response = await authorizeAt(origin, request);
    assert.equal(response.status, 201);
    assert.equal(((await response.json()) as FiscalResult).status, "rejected");
  });

  await withHTTP(new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger()), new InsecureLocalAuthorizer(), async (origin) => {
    const base = authorizationURL(origin);
    const consult = await fetch(`${base}/${encodeURIComponent(request.request_id)}/consult`, {
      method: "POST",
      headers: requestHeaders,
      body: JSON.stringify(request),
    });
    assert.equal(consult.status, 200);
    assert.equal(((await consult.json()) as FiscalResult).status, "not_found");

    assert.equal((await authorizeAt(origin, request)).status, 201);
    const changed = structuredClone(request);
    changed.recipient.document_number = "20999999999";
    const conflict = await authorizeAt(origin, changed);
    assert.equal(conflict.status, 409);
    assert.equal(((await conflict.json()) as FiscalProblem).code, "IDEMPOTENCY_KEY_REUSED");

    const invalid = await fetch(base, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(request) });
    assert.equal(invalid.status, 422);
  });

  await withHTTP(new FiscalService(new MockFiscalAuthority("timeout_before_processing"), new InMemoryFiscalLedger()), new InsecureLocalAuthorizer(), async (origin) => {
    const timeout = await authorizeAt(origin, request);
    assert.equal(timeout.status, 503);
    assert.equal(((await timeout.json()) as FiscalProblem).code, "AUTHORITY_TIMEOUT");
  });
});

test("unexpected application failures are 500 and never mislabeled as authentication errors", async () => {
  const broken: FiscalApplication = {
    async authorize() {
      throw new Error("database unavailable");
    },
    async consult() {
      throw new Error("database unavailable");
    },
  };
  await withHTTP(broken, new InsecureLocalAuthorizer(), async (origin) => {
    const response = await authorizeAt(origin, request);
    assert.equal(response.status, 500);
    assert.equal(((await response.json()) as FiscalProblem).code, "INTERNAL_ERROR");
  });
});

test("binds the signed correlation byte-for-byte to the header and body", async () => {
  const signedIdentity: InternalIdentity = {
    issuer: "pymes-v3",
    subject: "worker:fiscal",
    organizationId: request.organization_id,
    actorId: "user_primary",
    delegatedActorId: "user_delegated",
    roles: ["service"],
    requestId: "http-request-1",
    correlationId: request.correlation_id,
    tokenId: "token-1",
  };
  const signed: InternalAuthorizer = {
    async authorize() {
      return signedIdentity;
    },
  };
  await withHTTP(new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger()), signed, async (origin) => {
    const tamperedHeader = await fetch(authorizationURL(origin), {
      method: "POST",
      headers: { ...requestHeaders, "x-correlation-id": `${request.correlation_id}-tampered` },
      body: JSON.stringify(request),
    });
    assert.equal(tamperedHeader.status, 401);
    assert.equal(((await tamperedHeader.json()) as FiscalProblem).code, "UNAUTHORIZED_SERVICE");

    const tamperedBody = structuredClone(request);
    tamperedBody.correlation_id = `${request.correlation_id}-tampered`;
    const response = await fetch(authorizationURL(origin), {
      method: "POST",
      headers: requestHeaders,
      body: JSON.stringify(tamperedBody),
    });
    assert.equal(response.status, 422);
    assert.equal(((await response.json()) as FiscalProblem).code, "VALIDATION_ERROR");

    const bodyWithForgedActor = {
      ...request,
      actor_id: "user_attacker",
      delegated_actor_id: "user_forged",
    };
    const forgedActor = await fetch(authorizationURL(origin), {
      method: "POST",
      headers: requestHeaders,
      body: JSON.stringify(bodyWithForgedActor),
    });
    assert.equal(forgedActor.status, 422);
    assert.equal(((await forgedActor.json()) as FiscalProblem).code, "VALIDATION_ERROR");
  });
});

const requestHeaders = {
  "content-type": "application/json",
  "idempotency-key": request.idempotency_key,
  "x-correlation-id": request.correlation_id,
};

function authorizationURL(origin: string): string {
  return `${origin}/internal/v1/organizations/${request.organization_id}/authorizations`;
}

function authorizeAt(origin: string, value: FiscalRequest): Promise<Response> {
  return fetch(authorizationURL(origin), { method: "POST", headers: requestHeaders, body: JSON.stringify(value) });
}

async function withHTTP(
  application: FiscalApplication,
  authorizer: InternalAuthorizer,
  run: (origin: string) => Promise<void>,
  credentials?: CredentialApplication,
): Promise<void> {
  const server = createFiscalHTTPServer(
    application,
    authorizer,
    healthyRuntime,
    credentials,
  );
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  try {
    const { port } = server.address() as AddressInfo;
    await run(`http://127.0.0.1:${port}`);
  } finally {
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
}
