import { createPublicKey, verify, type JsonWebKey, type KeyObject } from "node:crypto";
import { FiscalError } from "../../fiscal/domain/fiscal.js";
import type { InternalAuthorizer, InternalIdentity } from "../../fiscal/ports/internal-authorizer.js";

interface Header {
  alg?: unknown;
  typ?: unknown;
  kid?: unknown;
}

interface Claims {
  iss?: unknown;
  aud?: unknown;
  sub?: unknown;
  org_id?: unknown;
  roles?: unknown;
  request_id?: unknown;
  correlation_id?: unknown;
  actor_id?: unknown;
  delegated_actor_id?: unknown;
  jti?: unknown;
  kid?: unknown;
  iat?: unknown;
  exp?: unknown;
}

interface Ed25519JWK {
  kty?: unknown;
  crv?: unknown;
  alg?: unknown;
  use?: unknown;
  key_ops?: unknown;
  kid?: unknown;
  x?: unknown;
}

interface JWKS {
  keys?: unknown;
}

const ed25519SPKIPrefix = Buffer.from("302a300506032b6570032100", "hex");

export class Ed25519JWTAuthorizer implements InternalAuthorizer {
  private readonly publicKeys: ReadonlyMap<string, KeyObject>;

  constructor(
    private readonly issuer: string,
    jwksJSON: string,
    private readonly now: () => Date = () => new Date(),
  ) {
    if (!nonEmpty(issuer) || !nonEmpty(jwksJSON)) throw new Error("internal JWT verifier is not configured");
    this.publicKeys = parseJWKS(jwksJSON);
  }

  async authorize(
    authorization: string | undefined,
    audience: "fiscal",
    expectedOrganizationId?: string,
    expectedCorrelationId?: string,
  ): Promise<InternalIdentity> {
    try {
      const token = bearerToken(authorization);
      const parts = token.split(".");
      if (parts.length !== 3) throw new Error("malformed JWT");
      const header = decodeJSON<Header>(parts[0]);
      const claims = decodeJSON<Claims>(parts[1]);
      if (header.alg !== "EdDSA" || header.typ !== "JWT" || !nonEmpty(header.kid)) {
        throw new Error("unexpected JWT header");
      }
      const publicKey = this.publicKeys.get(header.kid);
      if (publicKey === undefined) throw new Error("unknown JWT key");
      const signature = decodeBase64URL(parts[2]);
      if (!verify(null, Buffer.from(`${parts[0]}.${parts[1]}`), publicKey, signature)) {
        throw new Error("invalid JWT signature");
      }

      const now = Math.floor(this.now().getTime() / 1000);
      if (
        claims.iss !== this.issuer ||
        claims.aud !== audience ||
        claims.kid !== header.kid ||
        !opaqueReference(claims.sub) ||
        !opaqueReference(claims.org_id) ||
        (expectedOrganizationId !== undefined && claims.org_id !== expectedOrganizationId) ||
        !opaqueReference(claims.request_id) ||
        !opaqueReference(claims.correlation_id) ||
        (expectedCorrelationId !== undefined && claims.correlation_id !== expectedCorrelationId) ||
        !optionalOpaqueReference(claims.actor_id) ||
        !optionalOpaqueReference(claims.delegated_actor_id) ||
        (claims.delegated_actor_id !== undefined && claims.actor_id === undefined) ||
        !opaqueReference(claims.jti) ||
        !Array.isArray(claims.roles) ||
        !claims.roles.every(opaqueReference) ||
        !claims.roles.includes("service") ||
        !Number.isSafeInteger(claims.iat) ||
        !Number.isSafeInteger(claims.exp) ||
        (claims.iat as number) <= 0 ||
        (claims.iat as number) > now + 30 ||
        (claims.exp as number) <= now ||
        (claims.exp as number) <= (claims.iat as number) ||
        (claims.exp as number) - (claims.iat as number) > 300
      ) {
        throw new Error("invalid JWT claims");
      }

      return {
        issuer: claims.iss,
        subject: claims.sub,
        organizationId: claims.org_id,
        ...(claims.actor_id === undefined ? {} : { actorId: claims.actor_id }),
        ...(claims.delegated_actor_id === undefined ? {} : { delegatedActorId: claims.delegated_actor_id }),
        roles: claims.roles,
        requestId: claims.request_id,
        correlationId: claims.correlation_id,
        tokenId: claims.jti,
      };
    } catch {
      throw new FiscalError("UNAUTHORIZED_SERVICE");
    }
  }
}

export function legacyPublicKeyJWKS(publicKeyMaterial: string, keyID: string): string {
  if (!nonEmpty(publicKeyMaterial) || !nonEmpty(keyID)) {
    throw new Error("legacy local key material and key ID are required");
  }
  const publicKey = parsePublicKey(publicKeyMaterial);
  if (publicKey.asymmetricKeyType !== "ed25519") throw new Error("legacy local key must be Ed25519");
  const exported = publicKey.export({ format: "jwk" }) as JsonWebKey;
  if (!nonEmpty(exported.x)) throw new Error("legacy local key is missing its Ed25519 coordinate");
  return JSON.stringify({
    keys: [{ kty: "OKP", crv: "Ed25519", alg: "EdDSA", use: "sig", kid: keyID, x: exported.x }],
  });
}

function parseJWKS(value: string): ReadonlyMap<string, KeyObject> {
  let parsed: JWKS;
  try {
    parsed = JSON.parse(value) as JWKS;
  } catch {
    throw new Error("PYMES_INTERNAL_JWKS_JSON must be valid JSON");
  }
  if (!isRecord(parsed) || !Array.isArray(parsed.keys) || parsed.keys.length < 1) {
    throw new Error("PYMES_INTERNAL_JWKS_JSON must contain at least one key");
  }

  const keys = new Map<string, KeyObject>();
  for (const candidate of parsed.keys) {
    if (!isRecord(candidate)) throw new Error("PYMES_INTERNAL_JWKS_JSON contains an invalid key");
    const key = candidate as Ed25519JWK;
    if (
      key.kty !== "OKP" ||
      key.crv !== "Ed25519" ||
      key.alg !== "EdDSA" ||
      !nonEmpty(key.kid) ||
      !nonEmpty(key.x) ||
      (key.use !== undefined && key.use !== "sig") ||
      (key.key_ops !== undefined &&
        (!Array.isArray(key.key_ops) || !key.key_ops.every(nonEmpty) || !key.key_ops.includes("verify")))
    ) {
      throw new Error("PYMES_INTERNAL_JWKS_JSON contains a non-verification Ed25519 key");
    }
    if (keys.has(key.kid)) throw new Error("PYMES_INTERNAL_JWKS_JSON contains duplicate kid values");
    const raw = decodeBase64URL(key.x);
    if (raw.length !== 32) throw new Error("PYMES_INTERNAL_JWKS_JSON contains an invalid Ed25519 coordinate");
    const publicKey = createPublicKey({
      key: { kty: "OKP", crv: "Ed25519", x: key.x } as JsonWebKey,
      format: "jwk",
    });
    if (publicKey.asymmetricKeyType !== "ed25519") {
      throw new Error("PYMES_INTERNAL_JWKS_JSON contains a non-Ed25519 key");
    }
    keys.set(key.kid, publicKey);
  }
  return keys;
}

function parsePublicKey(material: string): KeyObject {
  if (material.includes("BEGIN PUBLIC KEY")) return createPublicKey(material);
  const raw = Buffer.from(material, "base64");
  if (raw.length !== 32) {
    throw new Error("PYMES_INTERNAL_PUBLIC_KEY_B64 must contain a 32-byte Ed25519 public key");
  }
  return createPublicKey({ key: Buffer.concat([ed25519SPKIPrefix, raw]), format: "der", type: "spki" });
}

function bearerToken(authorization: string | undefined): string {
  if (authorization === undefined || !authorization.startsWith("Bearer ")) throw new Error("missing bearer token");
  const token = authorization.slice("Bearer ".length);
  if (token.length < 1 || token.includes(" ")) throw new Error("invalid bearer token");
  return token;
}

function decodeJSON<T>(value: string): T {
  return JSON.parse(decodeBase64URL(value).toString("utf8")) as T;
}

function decodeBase64URL(value: string): Buffer {
  if (!nonEmpty(value) || !/^[A-Za-z0-9_-]+$/.test(value)) throw new Error("invalid base64url");
  const decoded = Buffer.from(value, "base64url");
  if (decoded.toString("base64url") !== value) throw new Error("non-canonical base64url");
  return decoded;
}

function nonEmpty(value: unknown): value is string {
  return typeof value === "string" && value.trim().length > 0;
}

function opaqueReference(value: unknown): value is string {
  return typeof value === "string" && /^[A-Za-z0-9:_./-]{1,255}$/.test(value);
}

function optionalOpaqueReference(value: unknown): value is string | undefined {
  return value === undefined || opaqueReference(value);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
