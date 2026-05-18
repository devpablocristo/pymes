// Genera tipos TypeScript a partir del openapi de Companion.
//
// Fuente: ../../companion/openapi.yaml (repo hermano en el monorepo local).
// Si el archivo local no existe, intenta como fallback la URL configurable
// PYMES_COMPANION_OPENAPI_URL — útil para CI donde companion puede no estar
// montado en disco.
//
// Output:
//   src/generated/companion.openapi.yaml  (copia textual del schema)
//   src/generated/companion.openapi.ts    (tipos generados por openapi-typescript)
//
// Mantenemos `companion.openapi.*` (no `pymes-ai.openapi.*`) porque Companion
// reemplaza a pymes-ai como backend del chat (ver
// modular-swinging-hummingbird plan, Fase 3.8).
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const __dirname = dirname(fileURLToPath(import.meta.url));
const frontendRoot = resolve(__dirname, "..");
const repoRoot = resolve(frontendRoot, "..");
const ecosystemRoot = resolve(repoRoot, "..");
const companionOpenapiLocal = resolve(ecosystemRoot, "companion", "openapi.yaml");
const outputDir = resolve(frontendRoot, "src", "generated");
const schemaPath = resolve(outputDir, "companion.openapi.yaml");
const typesPath = resolve(outputDir, "companion.openapi.ts");
const remoteUrl = process.env.PYMES_COMPANION_OPENAPI_URL;

async function exportSchema() {
  mkdirSync(outputDir, { recursive: true });

  if (existsSync(companionOpenapiLocal)) {
    const payload = readFileSync(companionOpenapiLocal, "utf-8");
    writeFileSync(schemaPath, payload.replace(/\r\n/g, "\n"), "utf-8");
    console.log(`schema: copied from ${companionOpenapiLocal}`);
    return;
  }

  if (remoteUrl) {
    const response = await fetch(remoteUrl);
    if (!response.ok) {
      throw new Error(`openapi_http_${response.status} for ${remoteUrl}`);
    }
    const payload = await response.text();
    writeFileSync(schemaPath, payload.replace(/\r\n/g, "\n"), "utf-8");
    console.log(`schema: fetched from ${remoteUrl}`);
    return;
  }

  throw new Error(
    `companion openapi not found at ${companionOpenapiLocal} and PYMES_COMPANION_OPENAPI_URL is unset`,
  );
}

(async () => {
  await exportSchema();

  const generateResult = spawnSync(
    "npx",
    ["openapi-typescript", schemaPath, "--output", typesPath],
    {
      cwd: frontendRoot,
      stdio: "inherit",
      env: process.env,
    },
  );
  if (generateResult.status !== 0) {
    process.exit(generateResult.status ?? 1);
  }

  // Forzar EOL estable para evitar diffs ruidosos entre entornos.
  const generated = readFileSync(typesPath, "utf-8");
  writeFileSync(typesPath, generated.replace(/\r\n/g, "\n"), "utf-8");
})();
