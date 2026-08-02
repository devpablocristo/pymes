import { createHash } from "node:crypto";
import type { FiscalRequest } from "../../usecases/domain/fiscal.js";

export function artifactReference(
  request: FiscalRequest,
  kind: string,
): string {
  const digest = createHash("sha256")
    .update(
      `${request.organization_id}\u0000${request.request_id}\u0000${request.snapshot_digest}\u0000${kind}`,
    )
    .digest("base64url")
    .slice(0, 43);
  return `fartifact_${digest}`;
}

export function artifactAAD(
  request: FiscalRequest,
  artifactId: string,
  kind: string,
): Uint8Array {
  return Buffer.from(
    `pymes-fiscal-v1\u0000${request.organization_id}\u0000${request.credential_ref}\u0000${request.environment}\u0000artifact\u0000${artifactId}\u0000${kind}`,
    "utf8",
  );
}
