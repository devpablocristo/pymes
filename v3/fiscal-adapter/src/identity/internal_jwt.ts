import { verify, type KeyObject } from "node:crypto";
import { FiscalError } from "../fiscal/usecases/domain/fiscal.js";
import type { InternalAuthorizer } from "../fiscal/handler.js";
import type { InternalIdentity } from "../fiscal/usecases.js";
import type {
  InternalClaims,
  JWTHeader,
} from "./internal_jwt/models/token.js";
import {
  bearerToken,
  decodeBase64URL,
  decodeJSON,
  nonEmpty,
  opaqueReference,
  optionalOpaqueReference,
} from "./internal_jwt/helpers/token.js";
import {
  legacyPublicKeyJWKS,
  parseJWKS,
} from "./internal_jwt/helpers/jwks.js";

export { legacyPublicKeyJWKS } from "./internal_jwt/helpers/jwks.js";

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
      const header = decodeJSON<JWTHeader>(parts[0]);
      const claims = decodeJSON<InternalClaims>(parts[1]);
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
