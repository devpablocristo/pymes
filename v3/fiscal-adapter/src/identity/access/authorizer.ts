import type { InternalAuthorizer, InternalIdentity } from "../../fiscal/ports/internal-authorizer.js";

// This adapter is intentionally available only when startup configuration
// opts into insecure local mode. It must never be selected implicitly.
export class InsecureLocalAuthorizer implements InternalAuthorizer {
  async authorize(
    _authorization: string | undefined,
    audience: "fiscal",
    expectedOrganizationId?: string,
    expectedCorrelationId?: string,
  ): Promise<InternalIdentity> {
    if (audience !== "fiscal") throw new Error("UNAUTHORIZED_SERVICE");
    return {
      issuer: "insecure-local",
      subject: "local-worker",
      organizationId: expectedOrganizationId ?? "catalog",
      roles: ["service"],
      requestId: "insecure-local",
      correlationId: expectedCorrelationId ?? "insecure-local",
      tokenId: "insecure-local",
    };
  }
}
