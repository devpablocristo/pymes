import assert from "node:assert/strict";
import { generateKeyPairSync, sign } from "node:crypto";
import test from "node:test";
import { FiscalError } from "../../src/fiscal/domain/fiscal.js";
import { Ed25519JWTAuthorizer } from "../../src/identity/access/ed25519-jwt-authorizer.js";

const now = new Date("2026-07-30T12:00:00.000Z");
const { publicKey, privateKey } = generateKeyPairSync("ed25519");
const rawPublicKey = publicKey.export({ format: "der", type: "spki" }).subarray(-32).toString("base64");
const authorizer = new Ed25519JWTAuthorizer("pymes-v3", rawPublicKey, () => now);

test("accepts a short Ed25519 workload JWT for the expected organization", async () => {
  const identity = await authorizer.authorize(`Bearer ${mint()}`, "fiscal", "org_a");
  assert.equal(identity.subject, "worker:fiscal");
  assert.equal(identity.organizationId, "org_a");
  assert.deepEqual(identity.roles, ["service"]);
});

test("rejects wrong audience, organization, expiry, excessive lifetime and signature", async () => {
  const cases = [
    mint({ aud: "accounting" }),
    mint({ org_id: "org_b" }),
    mint({ exp: epoch(now) }),
    mint({ exp: epoch(now) + 301 }),
    corruptSignature(mint()),
  ];
  for (const token of cases) {
    await assert.rejects(
      () => authorizer.authorize(`Bearer ${token}`, "fiscal", "org_a"),
      (error) => error instanceof FiscalError && error.code === "UNAUTHORIZED_SERVICE",
    );
  }
  await assert.rejects(
    () => authorizer.authorize(undefined, "fiscal", "org_a"),
    (error) => error instanceof FiscalError && error.code === "UNAUTHORIZED_SERVICE",
  );
});

function corruptSignature(token: string): string {
  const parts = token.split(".");
  const first = parts[2].at(0);
  parts[2] = `${first === "A" ? "B" : "A"}${parts[2].slice(1)}`;
  return parts.join(".");
}

function mint(overrides: Record<string, unknown> = {}): string {
  const issuedAt = epoch(now);
  const header = encode({ alg: "EdDSA", typ: "JWT", kid: "test-key" });
  const payload = encode({
    iss: "pymes-v3",
    aud: "fiscal",
    sub: "worker:fiscal",
    org_id: "org_a",
    roles: ["service"],
    request_id: "request-1",
    jti: "token-1",
    iat: issuedAt,
    exp: issuedAt + 300,
    kid: "test-key",
    ...overrides,
  });
  const input = `${header}.${payload}`;
  return `${input}.${sign(null, Buffer.from(input), privateKey).toString("base64url")}`;
}

function encode(value: unknown): string {
  return Buffer.from(JSON.stringify(value)).toString("base64url");
}

function epoch(value: Date): number {
  return Math.floor(value.getTime() / 1000);
}
