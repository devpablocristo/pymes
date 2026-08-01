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
    const reconciliationContext = {
      ...context,
      identity: { ...context.identity, requestId: "http-request-2", tokenId: "token-2" },
    };
    const reconciled = await secondService.consult(
      request.organization_id,
      request.request_id,
      undefined,
      reconciliationContext,
    );
    assert.equal(reconciled.status, "authorized");
    assert.equal(reconciled.cae?.length, 14);
    assert.equal(reconciled.idempotency_key, request.idempotency_key);
    assert.equal(reconciled.source_version, request.source_version);
    assert.equal(reconciled.snapshot_digest, request.snapshot_digest);
    assert.equal(reconciled.correlation_id, request.correlation_id);

    const repeated = await secondService.authorize(request, context);
    assert.equal(repeated.cae, reconciled.cae);
    const recorded = await secondStore.findByRequest(request.organization_id, request.request_id);
    assert.deepEqual(recorded?.audit, {
      correlationId: "before-restart",
      actorRef: "user_primary",
      delegatedActorRef: "user_delegated",
      workloadIssuer: "pymes-v3",
      workloadSubject: "worker:fiscal",
      workloadRequestId: "http-request-1",
      workloadTokenId: "token-1",
    });
    const count = await secondPool.query("SELECT count(*)::int AS count FROM fiscal.requests WHERE organization_id=$1", [request.organization_id]);
    assert.equal(count.rows[0].count, 1);
    const audit = await secondPool.query(
      `SELECT correlation_id,actor_ref,delegated_actor_ref,workload_subject,
              workload_request_id,workload_token_id,
              request ? 'actor_id' AS request_has_actor,
              result ? 'actor_id' AS result_has_actor
         FROM fiscal.requests
        WHERE organization_id=$1 AND request_id=$2`,
      [request.organization_id, request.request_id],
    );
    assert.deepEqual(audit.rows[0], {
      correlation_id: "before-restart",
      actor_ref: "user_primary",
      delegated_actor_ref: "user_delegated",
      workload_subject: "worker:fiscal",
      workload_request_id: "http-request-1",
      workload_token_id: "token-1",
      request_has_actor: false,
      result_has_actor: false,
    });
  } finally {
    await secondPool.end();
  }
});

test("PostgreSQL RLS isolates organizations while the repository scopes every transaction", { skip: databaseURL === undefined }, async () => {
  const admin = new Pool({ connectionString: databaseURL });
  const runtimeRole = "fiscal_runtime_identity_test";
  const runtimePassword = "fiscal-runtime-identity-test";
  try {
    await admin.query("TRUNCATE fiscal.requests, fiscal.mock_authorizations");
    await admin.query(`
      DO $$
      BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${runtimeRole}') THEN
          CREATE ROLE ${runtimeRole} LOGIN PASSWORD '${runtimePassword}'
            NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
        ELSE
          ALTER ROLE ${runtimeRole} LOGIN PASSWORD '${runtimePassword}'
            NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
        END IF;
      END
      $$`);
    await admin.query(`GRANT CONNECT ON DATABASE pymes_fiscal TO ${runtimeRole}`);
    await admin.query(`GRANT USAGE ON SCHEMA fiscal TO ${runtimeRole}`);
    await admin.query(`GRANT SELECT,INSERT,UPDATE,DELETE ON fiscal.requests TO ${runtimeRole}`);
    await admin.query(`GRANT EXECUTE ON FUNCTION fiscal.request_metrics() TO ${runtimeRole}`);

    const runtimeURL = new URL(databaseURL!);
    runtimeURL.username = runtimeRole;
    runtimeURL.password = runtimePassword;
    const runtime = new Pool({ connectionString: runtimeURL.toString() });
    try {
      const store = new PostgresFiscalStore(runtime);
      const service = new FiscalService(new MockFiscalAuthority(), store, now);
      const left = structuredClone(request);
      left.request_id = "fiscal:shared:1";
      left.idempotency_key = "fiscal:shared:1";
      left.correlation_id = "shared-correlation";
      const right = {
        ...structuredClone(left),
        organization_id: "org_other",
        credential_ref: "mock://credential/other",
      };
      await service.authorize(left, contextFor(left));
      await service.authorize(right, contextFor(right));

      const invisible = await runtime.query("SELECT count(*)::int AS count FROM fiscal.requests");
      assert.equal(invisible.rows[0].count, 0, "a connection without tenant context must fail closed");
      assert.equal((await store.findByRequest(left.organization_id, left.request_id))?.request.organization_id, "org_restart");
      assert.equal((await store.findByRequest(right.organization_id, right.request_id))?.request.organization_id, "org_other");

      const client = await runtime.connect();
      try {
        await client.query("BEGIN");
        await client.query("SELECT set_config('app.organization_id',$1,true)", [left.organization_id]);
        const visible = await client.query("SELECT organization_id FROM fiscal.requests");
        assert.deepEqual(visible.rows.map((row) => row.organization_id), [left.organization_id]);
        await client.query("ROLLBACK");
      } finally {
        client.release();
      }
      const metrics = await store.metrics();
      assert.equal(metrics.authorized, 2);
    } finally {
      await runtime.end();
    }
  } finally {
    await admin.end();
  }
});

const request: FiscalRequest = {
  request_id: "fiscal:restart:1",
  organization_id: "org_restart",
  idempotency_key: "fiscal:restart:1",
  correlation_id: "before-restart",
  source_version: 1,
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
const context = contextFor(request);
const now = () => new Date("2026-07-30T12:00:00.000Z");

function contextFor(value: Pick<FiscalRequest, "organization_id" | "idempotency_key" | "correlation_id">) {
  return {
    idempotencyKey: value.idempotency_key,
    correlationId: value.correlation_id,
    identity: {
      issuer: "pymes-v3",
      subject: "worker:fiscal",
      organizationId: value.organization_id,
      actorId: "user_primary",
      delegatedActorId: "user_delegated",
      roles: ["service"],
      requestId: "http-request-1",
      correlationId: value.correlation_id,
      tokenId: "token-1",
    },
  };
}
