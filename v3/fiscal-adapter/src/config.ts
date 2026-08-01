import type { MockScenario } from "./fiscal/mock_authority.js";
import { legacyPublicKeyJWKS } from "./identity/internal_jwt.js";

export interface Config {
  port: number;
  mode: "mock" | "arca";
  mockScenario: MockScenario;
  databaseURL: string;
  runtimeEnvironment: string;
  allowInsecureLocal: boolean;
  internalIssuer?: string;
  internalJWKSJSON: string;
  fiscalKMSKeyName: string;
  localKMSKeyB64?: string;
  homologationIssuerPattern?: string;
  productionIssuerPattern?: string;
  requestTimeoutMs: number;
}

const scenarios: MockScenario[] = ["authorized", "rejected", "timeout_before_processing", "response_lost_after_processing"];

export function loadConfig(environment: NodeJS.ProcessEnv = process.env): Config {
  const mode = environment.FISCAL_ADAPTER_MODE;
  if (mode !== "mock" && mode !== "arca") {
    throw new Error("FISCAL_ADAPTER_MODE must be mock or arca");
  }
  const databaseURL = environment.FISCAL_DATABASE_URL;
  if (databaseURL === undefined || databaseURL.length < 1) throw new Error("FISCAL_DATABASE_URL is required");
  const allowInsecureLocal = environment.FISCAL_ALLOW_INSECURE_LOCAL === "true";
  const allowLegacyLocal = environment.PYMES_ALLOW_LEGACY_INTERNAL_KEY_LOCAL === "true";
  const runtimeEnvironment = environment.PYMES_ENVIRONMENT ?? environment.NODE_ENV ?? "";
  if ((allowInsecureLocal || allowLegacyLocal) && runtimeEnvironment !== "development" && runtimeEnvironment !== "test") {
    throw new Error("local internal identity compatibility is forbidden outside development or test");
  }
  const internalIssuer = environment.PYMES_INTERNAL_ISSUER;
  let internalJWKSJSON = environment.PYMES_INTERNAL_JWKS_JSON ?? "";
  if (!allowInsecureLocal) {
    if (internalIssuer === undefined || internalIssuer.length < 1) throw new Error("PYMES_INTERNAL_ISSUER is required");
    if (internalJWKSJSON.length < 1 && allowLegacyLocal) {
      internalJWKSJSON = legacyPublicKeyJWKS(
        environment.PYMES_INTERNAL_PUBLIC_KEY_B64 ?? "",
        environment.PYMES_INTERNAL_KEY_ID ?? "",
      );
    }
    if (internalJWKSJSON.length < 1) throw new Error("PYMES_INTERNAL_JWKS_JSON is required");
  }
  const scenario = environment.FISCAL_MOCK_SCENARIO ?? "authorized";
  if (!scenarios.includes(scenario as MockScenario)) throw new Error("invalid FISCAL_MOCK_SCENARIO");
  const port = Number(environment.PORT ?? "8080");
  if (!Number.isSafeInteger(port) || port < 1 || port > 65535) throw new Error("invalid PORT");
  const requestTimeoutMs = Number(environment.FISCAL_ARCA_TIMEOUT_MS ?? "30000");
  if (!Number.isSafeInteger(requestTimeoutMs) || requestTimeoutMs < 1000 || requestTimeoutMs > 120000) {
    throw new Error("invalid FISCAL_ARCA_TIMEOUT_MS");
  }
  const productionKMS = environment.FISCAL_KMS_KEY_NAME;
  const testKMSKey = Buffer.alloc(32, 7).toString("base64");
  const localKMSKeyB64 =
    environment.FISCAL_LOCAL_KMS_KEY_B64 ??
    (runtimeEnvironment === "test" ? testKMSKey : undefined);
  if (localKMSKeyB64 !== undefined && runtimeEnvironment !== "development" && runtimeEnvironment !== "test") {
    throw new Error("local fiscal KMS is forbidden outside development or test");
  }
  const fiscalKMSKeyName =
    mode === "arca"
      ? productionKMS
      : "projects/local/locations/global/keyRings/local/cryptoKeys/fiscal";
  if (fiscalKMSKeyName === undefined || fiscalKMSKeyName.length < 1) {
    throw new Error("FISCAL_KMS_KEY_NAME is required in arca mode");
  }
  if (mode === "mock" && localKMSKeyB64 === undefined) {
    throw new Error("FISCAL_LOCAL_KMS_KEY_B64 is required in local mock mode");
  }
  const homologationIssuerPattern = environment.FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN;
  const productionIssuerPattern = environment.FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN;
  if (mode === "arca" && (homologationIssuerPattern === undefined || productionIssuerPattern === undefined)) {
    throw new Error("ARCA certificate issuer patterns are required in arca mode");
  }
  for (const pattern of [homologationIssuerPattern, productionIssuerPattern]) {
    if (pattern !== undefined) {
      if (pattern.length < 1 || pattern.length > 256) throw new Error("invalid ARCA issuer pattern");
      try {
        new RegExp(pattern, "i");
      } catch {
        throw new Error("invalid ARCA issuer pattern");
      }
    }
  }
  return {
    port,
    mode,
    mockScenario: scenario as MockScenario,
    databaseURL,
    runtimeEnvironment,
    allowInsecureLocal,
    internalIssuer,
    internalJWKSJSON,
    fiscalKMSKeyName,
    ...(localKMSKeyB64 === undefined ? {} : { localKMSKeyB64 }),
    ...(homologationIssuerPattern === undefined ? {} : { homologationIssuerPattern }),
    ...(productionIssuerPattern === undefined ? {} : { productionIssuerPattern }),
    requestTimeoutMs,
  };
}
