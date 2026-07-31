import { createPublicKey, verify, type KeyObject } from "node:crypto";
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
  jti?: unknown;
  iat?: unknown;
  exp?: unknown;
}

const ed25519SPKIPrefix = Buffer.from("302a300506032b6570032100", "hex");

export class Ed25519JWTAuthorizer implements InternalAuthorizer {
  private readonly publicKey: KeyObject;

  constructor(
    private readonly issuer: string,
    publicKeyMaterial: string,
    private readonly now: () => Date = () => new Date(),
  ) {
    if (issuer.length < 1 || publicKeyMaterial.length < 1) throw new Error("internal JWT verifier is not configured");
    this.publicKey = parsePublicKey(publicKeyMaterial);
  }

  async authorize(
    authorization: string | undefined,
    audience: "fiscal",
    expectedOrganizationId?: string,
  ): Promise<InternalIdentity> {
    try {
      const token = bearerToken(authorization);
      const parts = token.split(".");
      if (parts.length !== 3) throw new Error("malformed JWT");
      const header = decodeJSON<Header>(parts[0]);
      const claims = decodeJSON<Claims>(parts[1]);
      if (header.alg !== "EdDSA" || header.typ !== "JWT") throw new Error("unexpected JWT header");
      if (!verify(null, Buffer.from(`${parts[0]}.${parts[1]}`), this.publicKey, Buffer.from(parts[2], "base64url"))) {
        throw new Error("invalid JWT signature");
      }

      const now = Math.floor(this.now().getTime() / 1000);
      if (
        claims.iss !== this.issuer ||
        claims.aud !== audience ||
        !nonEmpty(claims.sub) ||
        !nonEmpty(claims.org_id) ||
        (expectedOrganizationId !== undefined && claims.org_id !== expectedOrganizationId) ||
        !nonEmpty(claims.request_id) ||
        !nonEmpty(claims.jti) ||
        !Array.isArray(claims.roles) ||
        !claims.roles.every(nonEmpty) ||
        !claims.roles.includes("service") ||
        !Number.isSafeInteger(claims.iat) ||
        !Number.isSafeInteger(claims.exp) ||
        (claims.iat as number) > now + 30 ||
        (claims.exp as number) <= now ||
        (claims.exp as number) - (claims.iat as number) > 300
      ) {
        throw new Error("invalid JWT claims");
      }

      return {
        issuer: claims.iss,
        subject: claims.sub,
        organizationId: claims.org_id,
        roles: claims.roles,
        requestId: claims.request_id,
        tokenId: claims.jti,
      };
    } catch {
      throw new FiscalError("UNAUTHORIZED_SERVICE");
    }
  }
}

function parsePublicKey(material: string): KeyObject {
  if (material.includes("BEGIN PUBLIC KEY")) return createPublicKey(material);
  const raw = Buffer.from(material, "base64");
  if (raw.length !== 32) throw new Error("PYMES_INTERNAL_PUBLIC_KEY_B64 must contain a 32-byte Ed25519 public key");
  return createPublicKey({ key: Buffer.concat([ed25519SPKIPrefix, raw]), format: "der", type: "spki" });
}

function bearerToken(authorization: string | undefined): string {
  if (authorization === undefined || !authorization.startsWith("Bearer ")) throw new Error("missing bearer token");
  const token = authorization.slice("Bearer ".length);
  if (token.length < 1 || token.includes(" ")) throw new Error("invalid bearer token");
  return token;
}

function decodeJSON<T>(value: string): T {
  return JSON.parse(Buffer.from(value, "base64url").toString("utf8")) as T;
}

function nonEmpty(value: unknown): value is string {
  return typeof value === "string" && value.length > 0;
}
