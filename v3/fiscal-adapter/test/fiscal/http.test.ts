import assert from "node:assert/strict";
import type { AddressInfo } from "node:net";
import test from "node:test";
import { InMemoryFiscalLedger } from "../../src/fiscal/companion/in-memory-ledger.js";
import { MockFiscalAuthority } from "../../src/fiscal/companion/mock-authority.js";
import { FiscalError, type FiscalProblem, type FiscalRequest, type FiscalResult } from "../../src/fiscal/domain/fiscal.js";
import { createFiscalHTTPServer, type FiscalApplication } from "../../src/fiscal/handler/http.js";
import type { InternalAuthorizer } from "../../src/fiscal/ports/internal-authorizer.js";
import type { FiscalRuntimeObserver } from "../../src/fiscal/ports/runtime-observer.js";
import { FiscalService } from "../../src/fiscal/usecases/fiscal-service.js";
import { InsecureLocalAuthorizer } from "../../src/identity/access/authorizer.js";

const request: FiscalRequest = {
  request_id: "fiscal:sale-http:1",
  organization_id: "org_http",
  credential_ref: "mock://credential/http",
  environment: "homologation",
  point_of_sale: 3,
  document_type: "FB",
  voucher_number: 8,
  issue_date: "2026-07-30",
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
  const headers = { "content-type": "application/json", "idempotency-key": request.request_id, "x-correlation-id": "http-test" };

  const authorize = await fetch(base, { method: "POST", headers, body: JSON.stringify(request) });
  assert.equal(authorize.status, 202);
  assert.equal(authorize.headers.get("cache-control"), "no-store");
  assert.equal(((await authorize.json()) as FiscalResult).status, "uncertain");

  const consult = await fetch(`${base}/${encodeURIComponent(request.request_id)}/consult`, { method: "POST", headers: { ...headers, "idempotency-key": `consult:${request.request_id}` }, body: JSON.stringify(request) });
  assert.equal(consult.status, 200);
  const result = (await consult.json()) as FiscalResult;
  assert.equal(result.status, "authorized");
  assert.equal(result.correlation_id, "http-test");
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

test("catalog is implemented and private routes reject invalid workload identity", async () => {
  await withHTTP(new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger()), new InsecureLocalAuthorizer(), async (origin) => {
    const catalog = await fetch(`${origin}/internal/v1/catalogs/document-types`);
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
    const catalog = await fetch(`${origin}/internal/v1/catalogs/document-types`);
    assert.equal(catalog.status, 401);
    const authorize = await fetch(`${origin}/internal/v1/organizations/${request.organization_id}/authorizations`, {
      method: "POST",
      headers: requestHeaders,
      body: JSON.stringify(request),
    });
    assert.equal(authorize.status, 401);
  });
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
      headers: { ...requestHeaders, "idempotency-key": `consult:${request.request_id}` },
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

const requestHeaders = {
  "content-type": "application/json",
  "idempotency-key": request.request_id,
  "x-correlation-id": "http-status-test",
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
): Promise<void> {
  const server = createFiscalHTTPServer(application, authorizer, healthyRuntime);
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  try {
    const { port } = server.address() as AddressInfo;
    await run(`http://127.0.0.1:${port}`);
  } finally {
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
}
