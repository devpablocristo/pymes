import assert from "node:assert/strict";
import test from "node:test";
import { Pool } from "pg";
import { MockFiscalAuthority } from "../../src/fiscal/mock_authority.js";
import type { FiscalRequest } from "../../src/fiscal/usecases/domain/fiscal.js";
import { PostgresFiscalStore } from "../../src/fiscal/repository.js";
import {
  FiscalService,
  type AuthorityDecision,
} from "../../src/fiscal/usecases.js";
import { PostgresCredentialRepository } from "../../src/credentials/repository.js";
import type {
  SealedValue,
  StoredCredential,
} from "../../src/credentials/usecases.js";

const databaseURL = process.env.FISCAL_DATABASE_TEST_URL;

test("PostgreSQL claim and CAS serialize concurrent authorization dispatch", { skip: databaseURL === undefined }, async () => {
  const pool = new Pool({ connectionString: databaseURL });
  try {
    await pool.query("TRUNCATE fiscal.requests, fiscal.mock_authorizations");
    const store = new PostgresFiscalStore(pool);
    let authorizeCalls = 0;
    const authority = {
      async authorize(): Promise<AuthorityDecision> {
        authorizeCalls += 1;
        await new Promise((resolve) => setTimeout(resolve, 40));
        return {
          status: "authorized" as const,
          cae: "12345678901234",
          cae_expires_on: "2026-08-09",
          result_code: "AUTHORIZED",
        };
      },
      async consult(): Promise<AuthorityDecision> {
        return { status: "not_found" };
      },
    };
    const services = Array.from(
      { length: 12 },
      () => new FiscalService(authority, store, now),
    );

    const results = await Promise.all(
      services.map((service) =>
        service.authorize(structuredClone(request), contextFor(request))
      ),
    );

    assert.equal(authorizeCalls, 1);
    assert.ok(results.every((result) => result.cae === "12345678901234"));
    const execution = await pool.query(
      `SELECT execution_state,execution_attempt,lease_token,lease_expires_at,
              dispatch_may_have_occurred
         FROM fiscal.requests
        WHERE organization_id=$1 AND request_id=$2`,
      [request.organization_id, request.request_id],
    );
    assert.deepEqual(execution.rows[0], {
      execution_state: "terminal",
      execution_attempt: "1",
      lease_token: null,
      lease_expires_at: null,
      dispatch_may_have_occurred: true,
    });

    const changed = structuredClone(request);
    changed.recipient.document_number = "20999999999";
    await assert.rejects(
      new FiscalService(authority, store, now).authorize(
        changed,
        contextFor(changed),
      ),
      (error: unknown) =>
        error instanceof Error &&
        "code" in error &&
        error.code === "IDEMPOTENCY_KEY_REUSED",
    );
    assert.equal(authorizeCalls, 1);
  } finally {
    await pool.end();
  }
});

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

test("PostgreSQL fiscal vault enforces tenant RLS across credentials, tickets and artifacts", { skip: databaseURL === undefined }, async () => {
  const admin = new Pool({ connectionString: databaseURL });
  const runtimeRole = "fiscal_vault_identity_test";
  const runtimePassword = "fiscal-vault-identity-test";
  try {
    await admin.query(
      "TRUNCATE fiscal.wsaa_tickets, fiscal.points_of_sale, fiscal.encrypted_artifacts, fiscal.credentials CASCADE",
    );
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
    await admin.query(
      `GRANT SELECT,INSERT,UPDATE,DELETE
         ON fiscal.credentials,fiscal.points_of_sale,fiscal.wsaa_tickets,fiscal.encrypted_artifacts
         TO ${runtimeRole}`,
    );

    const runtimeURL = new URL(databaseURL!);
    runtimeURL.username = runtimeRole;
    runtimeURL.password = runtimePassword;
    const runtime = new Pool({ connectionString: runtimeURL.toString() });
    try {
      const repository = new PostgresCredentialRepository(runtime);
      const left = credentialRecord("org_vault_left");
      const right = credentialRecord("org_vault_right");
      await repository.insertPending(left);
      await repository.insertPending(right);

      assert.equal(
        (await runtime.query("SELECT count(*)::int AS count FROM fiscal.credentials"))
          .rows[0].count,
        0,
        "queries without an organization context fail closed",
      );
      assert.equal(
        (await repository.find(left.organizationId, left.id))?.organizationId,
        left.organizationId,
      );
      assert.equal(
        (await repository.find(right.organizationId, right.id))?.organizationId,
        right.organizationId,
      );
      assert.equal(
        await repository.find(left.organizationId, "fcred_00000002"),
        undefined,
      );
      await repository.upsertPointOfSale({
        organizationId: left.organizationId,
        credentialId: left.id,
        environment: left.environment,
        number: 7,
        enabled: true,
        validatedAt: "2026-08-01T12:30:00.000Z",
      });
      assert.equal(
        (
          await repository.findPointOfSale(
            left.organizationId,
            left.id,
            left.environment,
            7,
          )
        )?.validatedAt,
        "2026-08-01T12:30:00.000Z",
      );

      await repository.saveTicket({
        organizationId: left.organizationId,
        credentialId: left.id,
        environment: left.environment,
        service: "wsfe",
        encryptedTicket: sealed,
        expiresAt: "2026-08-01T14:00:00.000Z",
      });
      await repository.saveTicket({
        organizationId: right.organizationId,
        credentialId: right.id,
        environment: right.environment,
        service: "wsfe",
        encryptedTicket: sealed,
        expiresAt: "2026-08-01T14:00:00.000Z",
      });
      assert.equal(
        (
          await repository.findTicket(
            left.organizationId,
            left.id,
            left.environment,
            "wsfe",
          )
        )?.organizationId,
        left.organizationId,
      );

      const client = await runtime.connect();
      try {
        await client.query("BEGIN");
        await client.query(
          "SELECT set_config('app.organization_id',$1,true)",
          [left.organizationId],
        );
        await assert.rejects(
          client.query(
            `INSERT INTO fiscal.encrypted_artifacts
               (organization_id,artifact_id,request_id,kind,encrypted_payload)
             VALUES($1,'fartifact_00000001','request','wsfe_authorization',$2)`,
            [right.organizationId, sealed],
          ),
          (error: unknown) =>
            typeof error === "object" &&
            error !== null &&
            "code" in error &&
            (error as { code: unknown }).code === "42501",
        );
        await client.query("ROLLBACK");
      } finally {
        client.release();
      }
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
  concept: "products",
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

const sealed: SealedValue = {
  format: "aes-256-gcm+kms-v1",
  ciphertext: Buffer.from("ciphertext").toString("base64"),
  encryptedDataKey: Buffer.alloc(32, 1).toString("base64"),
  iv: Buffer.alloc(12, 2).toString("base64"),
  authTag: Buffer.alloc(16, 3).toString("base64"),
  kmsKeyName: "projects/test/locations/global/keyRings/test/cryptoKeys/fiscal",
};

function credentialRecord(organizationId: string): StoredCredential {
  return {
    id: organizationId.endsWith("left")
      ? "fcred_00000001"
      : "fcred_00000002",
    organizationId,
    cuit: "20123456786",
    environment: "homologation",
    legalName: "Vault Test SA",
    commonName: "vault-test",
    status: "pending_certificate",
    idempotencyKey: "credential:idempotency:1",
    requestHash: "a".repeat(64),
    csrPem: "-----BEGIN CERTIFICATE REQUEST-----\ntest\n-----END CERTIFICATE REQUEST-----",
    encryptedPrivateKey: sealed,
    version: 1,
    createdAt: "2026-08-01T12:00:00.000Z",
    updatedAt: "2026-08-01T12:00:00.000Z",
  };
}
