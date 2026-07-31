import type { MockScenario } from "./fiscal/companion/mock-authority.js";

export interface Config {
  port: number;
  mode: "mock";
  mockScenario: MockScenario;
  databaseURL: string;
  allowInsecureLocal: boolean;
  internalIssuer?: string;
  internalPublicKey: string;
}

const scenarios: MockScenario[] = ["authorized", "rejected", "timeout_before_processing", "response_lost_after_processing"];

export function loadConfig(environment: NodeJS.ProcessEnv = process.env): Config {
  const mode = environment.FISCAL_ADAPTER_MODE;
  if (mode !== "mock") throw new Error("FISCAL_ADAPTER_MODE=mock is required while real ARCA is deferred");
  const databaseURL = environment.FISCAL_DATABASE_URL;
  if (databaseURL === undefined || databaseURL.length < 1) throw new Error("FISCAL_DATABASE_URL is required");
  const allowInsecureLocal = environment.FISCAL_ALLOW_INSECURE_LOCAL === "true";
  const internalIssuer = environment.PYMES_INTERNAL_ISSUER;
  const internalPublicKey = environment.PYMES_INTERNAL_PUBLIC_KEY_B64 ?? "";
  if (!allowInsecureLocal && (internalIssuer === undefined || internalIssuer.length < 1 || internalPublicKey.length < 1)) {
    throw new Error("PYMES_INTERNAL_ISSUER and PYMES_INTERNAL_PUBLIC_KEY_B64 are required");
  }
  const scenario = environment.FISCAL_MOCK_SCENARIO ?? "authorized";
  if (!scenarios.includes(scenario as MockScenario)) throw new Error("invalid FISCAL_MOCK_SCENARIO");
  const port = Number(environment.PORT ?? "8080");
  if (!Number.isSafeInteger(port) || port < 1 || port > 65535) throw new Error("invalid PORT");
  return { port, mode, mockScenario: scenario as MockScenario, databaseURL, allowInsecureLocal, internalIssuer, internalPublicKey };
}
