import {
  createPublicKey,
  type JsonWebKey,
  type KeyObject,
} from "node:crypto";
import type {
  Ed25519JWK,
  JWKS,
} from "../models/token.js";
import {
  decodeBase64URL,
  isRecord,
  nonEmpty,
} from "./token.js";

const ed25519SPKIPrefix = Buffer.from(
  "302a300506032b6570032100",
  "hex",
);

export function parseJWKS(value: string): ReadonlyMap<string, KeyObject> {
  let parsed: JWKS;
  try {
    parsed = JSON.parse(value) as JWKS;
  } catch {
    throw new Error("PYMES_INTERNAL_JWKS_JSON must be valid JSON");
  }
  if (
    !isRecord(parsed) ||
    !Array.isArray(parsed.keys) ||
    parsed.keys.length < 1
  ) {
    throw new Error("PYMES_INTERNAL_JWKS_JSON must contain at least one key");
  }

  const keys = new Map<string, KeyObject>();
  for (const candidate of parsed.keys) {
    if (!isRecord(candidate)) {
      throw new Error("PYMES_INTERNAL_JWKS_JSON contains an invalid key");
    }
    const key = candidate as Ed25519JWK;
    if (
      key.kty !== "OKP" ||
      key.crv !== "Ed25519" ||
      key.alg !== "EdDSA" ||
      !nonEmpty(key.kid) ||
      !nonEmpty(key.x) ||
      (key.use !== undefined && key.use !== "sig") ||
      (key.key_ops !== undefined &&
        (!Array.isArray(key.key_ops) ||
          !key.key_ops.every(nonEmpty) ||
          !key.key_ops.includes("verify")))
    ) {
      throw new Error(
        "PYMES_INTERNAL_JWKS_JSON contains a non-verification Ed25519 key",
      );
    }
    if (keys.has(key.kid)) {
      throw new Error(
        "PYMES_INTERNAL_JWKS_JSON contains duplicate kid values",
      );
    }
    const raw = decodeBase64URL(key.x);
    if (raw.length !== 32) {
      throw new Error(
        "PYMES_INTERNAL_JWKS_JSON contains an invalid Ed25519 coordinate",
      );
    }
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

export function legacyPublicKeyJWKS(
  publicKeyMaterial: string,
  keyID: string,
): string {
  if (!nonEmpty(publicKeyMaterial) || !nonEmpty(keyID)) {
    throw new Error("legacy local key material and key ID are required");
  }
  const publicKey = parsePublicKey(publicKeyMaterial);
  if (publicKey.asymmetricKeyType !== "ed25519") {
    throw new Error("legacy local key must be Ed25519");
  }
  const exported = publicKey.export({ format: "jwk" }) as JsonWebKey;
  if (!nonEmpty(exported.x)) {
    throw new Error(
      "legacy local key is missing its Ed25519 coordinate",
    );
  }
  return JSON.stringify({
    keys: [{
      kty: "OKP",
      crv: "Ed25519",
      alg: "EdDSA",
      use: "sig",
      kid: keyID,
      x: exported.x,
    }],
  });
}

function parsePublicKey(material: string): KeyObject {
  if (material.includes("BEGIN PUBLIC KEY")) {
    return createPublicKey(material);
  }
  const raw = Buffer.from(material, "base64");
  if (raw.length !== 32) {
    throw new Error(
      "PYMES_INTERNAL_PUBLIC_KEY_B64 must contain a 32-byte Ed25519 public key",
    );
  }
  return createPublicKey({
    key: Buffer.concat([ed25519SPKIPrefix, raw]),
    format: "der",
    type: "spki",
  });
}
