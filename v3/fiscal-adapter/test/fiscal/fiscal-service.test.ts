import assert from "node:assert/strict";
import test from "node:test";
import { InMemoryFiscalLedger } from "../../src/fiscal/companion/in-memory-ledger.js";
import { MockFiscalAuthority } from "../../src/fiscal/companion/mock-authority.js";
import { FiscalError, type FiscalRequest } from "../../src/fiscal/domain/fiscal.js";
import { FiscalService } from "../../src/fiscal/usecases/fiscal-service.js";

const request: FiscalRequest = {
  request_id: "fiscal:sale-1:1",
  organization_id: "org_a",
  credential_ref: "mock://credential/a",
  environment: "homologation",
  point_of_sale: 1,
  document_type: "FA",
  voucher_number: 42,
  issue_date: "2026-07-30",
  currency: "ARS",
  totals: { net: "100", vat: "21", exempt: "0", total: "121.00" },
  recipient: { document_type: "CUIT", document_number: "20123456789", vat_condition: "registered" },
  lines: [{ description: "Servicio", quantity: "1", unit_price: "100", vat_rate: "21", net: "100" }],
  snapshot_digest: "a".repeat(64),
};
const context = { idempotencyKey: "fiscal:sale-1:1", correlationId: "sale-1" };
const now = () => new Date("2026-07-30T12:00:00.000Z");

test("authorizes the exact number supplied by Pymes", async () => {
  const authority = new MockFiscalAuthority("authorized");
  const service = new FiscalService(authority, new InMemoryFiscalLedger(), now);
  const result = await service.authorize(request, context);
  assert.equal(result.status, "authorized");
  assert.equal(result.cae?.length, 14);
  assert.equal(result.snapshot_digest, request.snapshot_digest);
  assert.equal(authority.received[0].voucher_number, 42);
  assert.equal(authority.received[0].point_of_sale, 1);
});

test("response loss becomes uncertain and exact consultation recovers the CAE", async () => {
  const authority = new MockFiscalAuthority("response_lost_after_processing");
  const service = new FiscalService(authority, new InMemoryFiscalLedger(), now);
  const first = await service.authorize(request, context);
  assert.equal(first.status, "uncertain");
  const reconciled = await service.consult(request.organization_id, request.request_id, request, { idempotencyKey: `consult:${context.idempotencyKey}`, correlationId: context.correlationId });
  assert.equal(reconciled.status, "authorized");
  assert.equal(reconciled.cae?.length, 14);
  assert.equal(authority.received.length, 1, "consultation must not issue a second voucher");
});

test("exact retries return the original result and changed payloads are rejected", async () => {
  const authority = new MockFiscalAuthority();
  const service = new FiscalService(authority, new InMemoryFiscalLedger(), now);
  const first = await service.authorize(request, context);
  const repeated = await service.authorize(structuredClone(request), context);
  assert.deepEqual(repeated, first);
  assert.equal(authority.received.length, 1);

  const changed = structuredClone(request);
  changed.recipient.document_number = "20999999999";
  await assert.rejects(() => service.authorize(changed, context), (error) => error instanceof FiscalError && error.code === "IDEMPOTENCY_KEY_REUSED");
});

test("organizations are isolated even when request and idempotency IDs coincide", async () => {
  const service = new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger(), now);
  const left = await service.authorize(request, context);
  const rightRequest = { ...structuredClone(request), organization_id: "org_b", credential_ref: "mock://credential/b" };
  const right = await service.authorize(rightRequest, context);
  assert.equal(left.status, "authorized");
  assert.equal(right.status, "authorized");
  assert.notEqual(left.cae, right.cae);
});

test("timeout before processing is distinguishable from an uncertain authorization", async () => {
  const service = new FiscalService(new MockFiscalAuthority("timeout_before_processing"), new InMemoryFiscalLedger(), now);
  await assert.rejects(() => service.authorize(request, context), (error) => error instanceof FiscalError && error.code === "AUTHORITY_TIMEOUT");
});

test("mock accepts A/B/C and their credit/debit notes with explicit association", async () => {
  const authority = new MockFiscalAuthority();
  const service = new FiscalService(authority, new InMemoryFiscalLedger(), now);
  const types = ["FA", "FB", "FC", "NCA", "NCB", "NCC", "NDA", "NDB", "NDC"] as const;
  for (const [index, documentType] of types.entries()) {
    const current = structuredClone(request);
    current.request_id = `fiscal:document-${index}:1`;
    current.document_type = documentType;
    current.voucher_number = index + 1;
    if (documentType.startsWith("NC") || documentType.startsWith("ND")) {
      current.associated_voucher = { point_of_sale: 1, document_type: documentType.endsWith("A") ? "FA" : documentType.endsWith("B") ? "FB" : "FC", voucher_number: 1, issue_date: current.issue_date };
    }
    const result = await service.authorize(current, { idempotencyKey: current.request_id, correlationId: current.request_id });
    assert.equal(result.status, "authorized", documentType);
  }
});

test("foreign currency requires an explicit positive exchange rate", async () => {
  const service = new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger(), now);
  const usd = { ...structuredClone(request), request_id: "fiscal:usd:1", currency: "USD" };
  await assert.rejects(() => service.authorize(usd, { idempotencyKey: usd.request_id, correlationId: usd.request_id }), (error) => error instanceof FiscalError && error.code === "VALIDATION_ERROR");
  usd.exchange_rate = "1250.50";
  assert.equal((await service.authorize(usd, { idempotencyKey: usd.request_id, correlationId: usd.request_id })).status, "authorized");

  const eur = { ...structuredClone(request), request_id: "fiscal:eur:1", currency: "EUR", exchange_rate: "1399.20" };
  assert.equal((await service.authorize(eur, { idempotencyKey: eur.request_id, correlationId: eur.request_id })).status, "authorized");
});

test("accepts only the supported VAT rates and rejects unknown currencies", async () => {
  const service = new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger(), now);
  for (const [index, vatRate] of ["0", "2.5", "5", "10.5", "21.0", "27"].entries()) {
    const current = structuredClone(request);
    current.request_id = `fiscal:vat-${index}:1`;
    current.voucher_number = 100 + index;
    current.lines[0].vat_rate = vatRate;
    assert.equal((await service.authorize(current, { idempotencyKey: current.request_id, correlationId: current.request_id })).status, "authorized");
  }
  const invalidVAT = structuredClone(request);
  invalidVAT.request_id = "fiscal:vat-invalid:1";
  invalidVAT.lines[0].vat_rate = "19";
  await assert.rejects(() => service.authorize(invalidVAT, { idempotencyKey: invalidVAT.request_id, correlationId: invalidVAT.request_id }), FiscalError);

  const invalidCurrency = { ...structuredClone(request), request_id: "fiscal:gbp:1", currency: "GBP", exchange_rate: "1" };
  await assert.rejects(() => service.authorize(invalidCurrency, { idempotencyKey: invalidCurrency.request_id, correlationId: invalidCurrency.request_id }), FiscalError);
});

test("rejected and not-found mock outcomes remain explicit", async () => {
  const rejected = new FiscalService(new MockFiscalAuthority("rejected"), new InMemoryFiscalLedger(), now);
  assert.equal((await rejected.authorize(request, context)).status, "rejected");

  const missing = new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger(), now);
  const result = await missing.consult(request.organization_id, request.request_id, request, {
    idempotencyKey: "consult:fiscal:sale-1:1",
    correlationId: "missing",
  });
  assert.equal(result.status, "not_found");
});
