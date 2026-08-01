import type { InternalAuthorizer } from "../fiscal/handler.js";
import type { InternalIdentity } from "../fiscal/usecases.js";
import { insecureLocalIdentity } from "./insecure_local/models/identity.js";
import { assertFiscalAudience } from "./insecure_local/helpers/audience.js";

// This adapter is intentionally available only when startup configuration
// opts into insecure local mode. It must never be selected implicitly.
export class InsecureLocalAuthorizer implements InternalAuthorizer {
  async authorize(
    _authorization: string | undefined,
    audience: "fiscal",
    expectedOrganizationId?: string,
    expectedCorrelationId?: string,
  ): Promise<InternalIdentity> {
    assertFiscalAudience(audience);
    return {
      ...insecureLocalIdentity,
      roles: [...insecureLocalIdentity.roles],
      organizationId: expectedOrganizationId ?? "catalog",
      correlationId: expectedCorrelationId ?? "insecure-local",
    };
  }
}
