import type { CredentialEnvironment } from "../../usecases/domain/credential.js";

export type TrustedIssuerPatterns = Record<CredentialEnvironment, RegExp>;
