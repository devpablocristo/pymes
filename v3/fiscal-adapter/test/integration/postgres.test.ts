import assert from "node:assert/strict";
import test from "node:test";
import { Pool } from "pg";
import { MockFiscalAuthority } from "../../src/fiscal/companion/mock-authority.js";
import type { FiscalRequest } from "../../src/fiscal/domain/fiscal.js";
import { PostgresFiscalStore } from "../../src/fiscal/repository/postgres-store.js";
import { FiscalService } from "../../src/fiscal/usecases/fiscal-service.js";

const databaseURL = process.env.FISCAL_DATABASE_TEST_URL;

test("PostgreSQL preserves uncertain authorization and idempotency across reconstruction", { skip: databaseURL === undefined }, async () => {
  const firstPool = new Pool({ connectionString: databaseURL });
  await firstPool.query("TRUNCATE fiscal.requests, fiscal.mock_authorizations");
  const firstStore = new PostgresFiscalStore(firstPool);
  const firstService = new FiscalService(new MockFiscalAuthority("response_lost_after_processing", firstStore), firstStore, now);
  const uncertain = await firstService.authorize(request, context);
  assert.equal(uncertain.status, "uncertain");
  await firstPool.end();

  const secondPool = new Pool({ connectionString: databaseURL });
  try {
    const secondStore = new PostgresFiscalStore(secondPool);
    const secondService = new FiscalService(new MockFiscalAuthority("authorized", secondStore), secondStore, now);
    const reconciled = await secondService.consult(request.organization_id, request.request_id, undefined, {
      idempotencyKey: "consult:fiscal:restart:1",
      correlationId: "after-restart",
    });
    assert.equal(reconciled.status, "authorized");
    assert.equal(reconciled.cae?.length, 14);
    assert.equal(reconciled.correlation_id, "after-restart");

    const repeated = await secondService.authorize(request, context);
    assert.equal(repeated.cae, reconciled.cae);
    const count = await secondPool.query("SELECT count(*)::int AS count FROM fiscal.requests WHERE organization_id=$1", [request.organization_id]);
    assert.equal(count.rows[0].count, 1);
  } finally {
    await secondPool.end();
  }
});

const request: FiscalRequest = {
  request_id: "fiscal:restart:1",
  organization_id: "org_restart",
  credential_ref: "mock://credential/restart",
  environment: "homologation",
  point_of_sale: 4,
  document_type: "FA",
  voucher_number: 9,
  issue_date: "2026-07-30",
  currency: "ARS",
  totals: { net: "100", vat: "21", exempt: "0", total: "121" },
  recipient: { document_type: "CUIT", document_number: "20123456789", vat_condition: "registered" },
  lines: [{ description: "Servicio", quantity: "1", unit_price: "100", vat_rate: "21", net: "100" }],
  snapshot_digest: "c".repeat(64),
};
const context = { idempotencyKey: request.request_id, correlationId: "before-restart" };
const now = () => new Date("2026-07-30T12:00:00.000Z");
