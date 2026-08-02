import { CredentialError } from "../../usecases/domain/credential.js";

export function decodeLocalKMSKey(value: string): Uint8Array {
  const key = Buffer.from(value, "base64");
  if (key.byteLength !== 32 || key.toString("base64") !== value) {
    throw new CredentialError("VALIDATION_ERROR", "invalid local KMS key");
  }
  return key;
}
