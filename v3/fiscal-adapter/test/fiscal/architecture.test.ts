import assert from "node:assert/strict";
import { readFile, stat } from "node:fs/promises";
import test from "node:test";

const adapters = [
  "src/fiscal/handler",
  "src/fiscal/repository",
  "src/fiscal/arca",
  "src/fiscal/mock_authority",
  "src/fiscal/in_memory_ledger",
  "src/credentials/handler",
  "src/credentials/repository",
  "src/credentials/kms",
  "src/credentials/local_kms",
  "src/credentials/csr",
  "src/credentials/certificate",
  "src/credentials/credential_id",
  "src/identity/internal_jwt",
  "src/identity/insecure_local",
] as const;

test("every Fiscal adapter has non-empty models and helpers directories", async () => {
  for (const adapter of adapters) {
    for (const concern of ["models", "helpers"] as const) {
      const directory = `${adapter}/${concern}`;
      assert.equal(
        (await stat(directory)).isDirectory(),
        true,
        `${directory} must exist`,
      );
      const marker = await readFile(
        `${directory}/${expectedFile(adapter, concern)}`,
        "utf8",
      );
      assert.ok(marker.trim().length > 0, `${directory} must be useful`);
    }
  }
});

test("legacy horizontal Fiscal layers cannot return", async () => {
  for (const forbidden of [
    "src/fiscal/domain",
    "src/fiscal/ports",
    "src/fiscal/companion",
    "src/fiscal/handler/http.ts",
    "src/fiscal/repository/postgres-store.ts",
    "src/fiscal/usecases/fiscal-service.ts",
    "src/identity/access",
  ]) {
    await assert.rejects(stat(forbidden), { code: "ENOENT" }, forbidden);
  }
});

test("domain and use cases stay independent from provider infrastructure", async () => {
  const domain = await readFile(
    "src/fiscal/usecases/domain/fiscal.ts",
    "utf8",
  );
  const usecases = await readFile("src/fiscal/usecases.ts", "utf8");
  for (const forbidden of [
    "node:http",
    "pg",
    "node-forge",
    "@google-cloud",
    "@devpablocristo/arca-facturacion",
    "../handler",
    "../repository",
    "../arca",
  ]) {
    assert.equal(domain.includes(forbidden), false, `domain imports ${forbidden}`);
    assert.equal(
      usecases.includes(forbidden),
      false,
      `use cases import ${forbidden}`,
    );
  }
});

function expectedFile(
  adapter: (typeof adapters)[number],
  concern: "models" | "helpers",
): string {
  const files: Record<string, [string, string]> = {
    "src/fiscal/handler": ["http.ts", "http.ts"],
    "src/fiscal/repository": ["rows.ts", "mappers.ts"],
    "src/fiscal/arca": ["sdk.ts", "mapping.ts"],
    "src/fiscal/mock_authority": ["scenario.ts", "decisions.ts"],
    "src/fiscal/in_memory_ledger": ["state.ts", "records.ts"],
    "src/credentials/handler": ["http.ts", "http.ts"],
    "src/credentials/repository": ["rows.ts", "mappers.ts"],
    "src/credentials/kms": ["client.ts", "aes.ts"],
    "src/credentials/local_kms": ["ciphertext.ts", "key.ts"],
    "src/credentials/csr": ["subject.ts", "distinguished-name.ts"],
    "src/credentials/certificate": ["config.ts", "x509.ts"],
    "src/credentials/credential_id": ["constants.ts", "encoding.ts"],
    "src/identity/internal_jwt": ["token.ts", "token.ts"],
    "src/identity/insecure_local": ["identity.ts", "audience.ts"],
  };
  return files[adapter]![concern === "models" ? 0 : 1];
}
