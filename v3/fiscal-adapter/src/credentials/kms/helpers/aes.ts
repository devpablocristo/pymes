import {
  createCipheriv,
  createDecipheriv,
  timingSafeEqual,
} from "node:crypto";
import { CredentialError } from "../../usecases/domain/credential.js";

export interface AESGCMCiphertext {
  ciphertext: Uint8Array;
  iv: Uint8Array;
  authTag: Uint8Array;
}

export function encryptAESGCM(
  plaintext: Uint8Array,
  key: Uint8Array,
  iv: Uint8Array,
  aad: Uint8Array,
): AESGCMCiphertext {
  validateShape(key, iv);
  const cipher = createCipheriv("aes-256-gcm", key, iv);
  cipher.setAAD(aad);
  const ciphertext = Buffer.concat([
    cipher.update(plaintext),
    cipher.final(),
  ]);
  return {
    ciphertext,
    iv: Buffer.from(iv),
    authTag: cipher.getAuthTag(),
  };
}

export function decryptAESGCM(
  encrypted: AESGCMCiphertext,
  key: Uint8Array,
  aad: Uint8Array,
): Uint8Array {
  validateShape(key, encrypted.iv);
  if (encrypted.authTag.byteLength !== 16) {
    throw new CredentialError("CREDENTIAL_NOT_READY", "invalid authentication tag");
  }
  try {
    const decipher = createDecipheriv("aes-256-gcm", key, encrypted.iv);
    decipher.setAAD(aad);
    decipher.setAuthTag(encrypted.authTag);
    return Buffer.concat([
      decipher.update(encrypted.ciphertext),
      decipher.final(),
    ]);
  } catch {
    throw new CredentialError("CREDENTIAL_NOT_READY", "encrypted material authentication failed");
  }
}

export function decodeBase64(value: string, field: string): Uint8Array {
  if (!/^[A-Za-z0-9+/]+={0,2}$/.test(value) || value.length % 4 !== 0) {
    throw new CredentialError("CREDENTIAL_NOT_READY", `invalid ${field}`);
  }
  const decoded = Buffer.from(value, "base64");
  const canonical = decoded.toString("base64");
  const left = Buffer.from(canonical);
  const right = Buffer.from(value);
  if (left.length !== right.length || !timingSafeEqual(left, right)) {
    throw new CredentialError("CREDENTIAL_NOT_READY", `invalid ${field}`);
  }
  return decoded;
}

export function kmsBytes(
  value: Uint8Array | string | null | undefined,
  field: string,
): Uint8Array {
  if (value === undefined || value === null) {
    throw new CredentialError("CREDENTIAL_NOT_READY", `KMS omitted ${field}`);
  }
  if (typeof value === "string") return Buffer.from(value, "base64");
  return Buffer.from(value);
}

function validateShape(key: Uint8Array, iv: Uint8Array): void {
  if (key.byteLength !== 32 || iv.byteLength !== 12) {
    throw new CredentialError("CREDENTIAL_NOT_READY", "invalid envelope shape");
  }
}
