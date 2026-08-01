import assert from "node:assert/strict";
import { generateKeyPairSync, sign, type JsonWebKey, type KeyPairKeyObjectResult } from "node:crypto";
import test from "node:test";
import { FiscalError } from "../../src/fiscal/domain/fiscal.js";
import {
  Ed25519JWTAuthorizer,
  legacyPublicKeyJWKS,
} from "../../src/identity/access/ed25519-jwt-authorizer.js";

const now = new Date("2026-07-30T12:00:00.000Z");
const current = generateKeyPairSync("ed25519");
const previous = generateKeyPairSync("ed25519");
const currentKeyID = "2026-07-current";
const previousKeyID = "2026-06-previous";
const jwksJSON = jwks([
  publicJWK(previous, previousKeyID),
  publicJWK(current, currentKeyID),
]);
const authorizer = new Ed25519JWTAuthorizer("pymes-v3", jwksJSON, () => now);

test("accepts current and previous kid during an overlapping rotation", async () => {
  for (const [pair, keyID] of [[current, currentKeyID], [previous, previousKeyID]] as const) {
    const identity = await authorizer.authorize(`Bearer ${mint(pair, keyID)}`, "fiscal", "org_a");
    assert.equal(identity.subject, "worker:fiscal");
    assert.equal(identity.organizationId, "org_a");
    assert.deepEqual(identity.roles, ["service"]);
    assert.equal(identity.requestId, "request-1");
    assert.equal(identity.correlationId, "correlation-1");
    assert.equal(identity.actorId, "user_primary");
    assert.equal(identity.delegatedActorId, "user_delegated");
    assert.equal(identity.tokenId, "token-1");
  }
  const workloadOnly = await authorizer.authorize(
    `Bearer ${mint(current, currentKeyID, { actor_id: undefined, delegated_actor_id: undefined })}`,
    "fiscal",
    "org_a",
    "correlation-1",
  );
  assert.equal(workloadOnly.actorId, undefined);
  assert.equal(workloadOnly.delegatedActorId, undefined);
});

test("rejects missing or unknown kid, wrong algorithm and a kid/signature mismatch", async () => {
  const cases = [
    mint(current, currentKeyID, {}, { kid: undefined }),
    mint(current, "unknown"),
    mint(current, currentKeyID, {}, { alg: "RS256" }),
    mint(previous, currentKeyID),
    corruptSignature(mint(current, currentKeyID)),
    tamperCorrelation(mint(current, currentKeyID)),
  ];
  for (const token of cases) await rejects(token);
});

test("rejects missing or invalid required workload claims", async () => {
  const invalidClaims: Record<string, unknown>[] = [
    { iss: "other" },
    { aud: "accounting" },
    { org_id: "org_b" },
    { org_id: "" },
    { sub: "" },
    { roles: ["viewer"] },
    { roles: ["service", ""] },
    { request_id: "" },
    { correlation_id: "" },
    { correlation_id: "contains whitespace" },
    { actor_id: "" },
    { actor_id: "user@example.com" },
    { delegated_actor_id: "" },
    { actor_id: undefined, delegated_actor_id: "user_delegated" },
    { kid: "different-payload-key" },
    { jti: "" },
    { iat: 0 },
    { iat: epoch(now) + 31 },
    { exp: epoch(now) },
    { exp: epoch(now) - 1 },
    { exp: epoch(now) + 301 },
    { iat: epoch(now) + 1, exp: epoch(now) + 1 },
  ];
  for (const claims of invalidClaims) await rejects(mint(current, currentKeyID, claims));
  await assert.rejects(
    () => authorizer.authorize(undefined, "fiscal", "org_a"),
    unauthorized,
  );
  await assert.rejects(
    () => authorizer.authorize(`Bearer ${mint(current, currentKeyID)}`, "fiscal", "org_a", "other-correlation"),
    unauthorized,
  );
});

test("rejects malformed, duplicate and non-Ed25519 JWKS entries", () => {
  const valid = publicJWK(current, currentKeyID);
  const invalidSets = [
    "",
    "{",
    JSON.stringify({ keys: [] }),
    jwks([valid, valid]),
    jwks([{ ...valid, kid: "" }]),
    jwks([{ ...valid, alg: "RS256" }]),
    jwks([{ ...valid, kty: "RSA" }]),
    jwks([{ ...valid, crv: "X25519" }]),
    jwks([{ ...valid, use: "enc" }]),
    jwks([{ ...valid, key_ops: ["sign"] }]),
    jwks([{ ...valid, x: "AA" }]),
  ];
  for (const candidate of invalidSets) {
    assert.throws(() => new Ed25519JWTAuthorizer("pymes-v3", candidate, () => now));
  }
});

test("builds a standard single-key JWKS only for explicit legacy local configuration", async () => {
  const raw = current.publicKey.export({ format: "der", type: "spki" }).subarray(-32).toString("base64");
  const local = new Ed25519JWTAuthorizer(
    "pymes-v3",
    legacyPublicKeyJWKS(raw, currentKeyID),
    () => now,
  );
  await local.authorize(`Bearer ${mint(current, currentKeyID)}`, "fiscal", "org_a");
  assert.throws(() => legacyPublicKeyJWKS(raw, ""));
});

async function rejects(token: string): Promise<void> {
  await assert.rejects(
    () => authorizer.authorize(`Bearer ${token}`, "fiscal", "org_a"),
    unauthorized,
  );
}

function unauthorized(error: unknown): boolean {
  return error instanceof FiscalError && error.code === "UNAUTHORIZED_SERVICE";
}

function corruptSignature(token: string): string {
  const parts = token.split(".");
  const first = parts[2].at(0);
  parts[2] = `${first === "A" ? "B" : "A"}${parts[2].slice(1)}`;
  return parts.join(".");
}

function tamperCorrelation(token: string): string {
  const parts = token.split(".");
  const claims = JSON.parse(Buffer.from(parts[1], "base64url").toString("utf8")) as Record<string, unknown>;
  claims.correlation_id = "tampered-correlation";
  parts[1] = encode(claims);
  return parts.join(".");
}

function mint(
  pair: KeyPairKeyObjectResult,
  keyID: string,
  overrides: Record<string, unknown> = {},
  headerOverrides: Record<string, unknown> = {},
): string {
  const issuedAt = epoch(now);
  const header = encode({ alg: "EdDSA", typ: "JWT", kid: keyID, ...headerOverrides });
  const payload = encode({
    iss: "pymes-v3",
    aud: "fiscal",
    sub: "worker:fiscal",
    org_id: "org_a",
    roles: ["service"],
    request_id: "request-1",
    correlation_id: "correlation-1",
    actor_id: "user_primary",
    delegated_actor_id: "user_delegated",
    jti: "token-1",
    iat: issuedAt,
    exp: issuedAt + 300,
    kid: keyID,
    ...overrides,
  });
  const input = `${header}.${payload}`;
  return `${input}.${sign(null, Buffer.from(input), pair.privateKey).toString("base64url")}`;
}

function publicJWK(pair: KeyPairKeyObjectResult, keyID: string): Record<string, unknown> {
  const exported = pair.publicKey.export({ format: "jwk" }) as JsonWebKey;
  return {
    kty: "OKP",
    crv: "Ed25519",
    alg: "EdDSA",
    use: "sig",
    key_ops: ["verify"],
    kid: keyID,
    x: exported.x,
  };
}

function jwks(keys: Record<string, unknown>[]): string {
  return JSON.stringify({ keys });
}

function encode(value: unknown): string {
  return Buffer.from(JSON.stringify(value)).toString("base64url");
}

function epoch(value: Date): number {
  return Math.floor(value.getTime() / 1000);
}
