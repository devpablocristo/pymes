import { randomBytes } from "node:crypto";
import type { CredentialIDGenerator } from "./usecases.js";
import { CREDENTIAL_ID_BYTES } from "./credential_id/models/constants.js";
import { credentialID } from "./credential_id/helpers/encoding.js";

export class RandomCredentialIDGenerator implements CredentialIDGenerator {
  constructor(
    private readonly entropy: (size: number) => Uint8Array = randomBytes,
  ) {}

  next(): string {
    return credentialID(this.entropy(CREDENTIAL_ID_BYTES));
  }
}
