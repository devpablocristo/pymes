import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const __dirname = dirname(fileURLToPath(import.meta.url));
const frontendRoot = resolve(__dirname, "..");
const repoRoot = resolve(frontendRoot, "..");
const exportScript = resolve(repoRoot, "ai", "scripts", "export_openapi.py");
const outputDir = resolve(frontendRoot, "src", "generated");
const schemaPath = resolve(outputDir, "pymes-ai.openapi.json");
const typesPath = resolve(outputDir, "pymes-ai.openapi.ts");
const schemaUrl = process.env.PYMES_AI_OPENAPI_URL ?? "http://localhost:8200/openapi.json";

async function exportSchema() {
  mkdirSync(outputDir, { recursive: true });
  try {
    const response = await fetch(schemaUrl);
    if (!response.ok)
      throw new Error(`openapi_http_${response.status}`);
    const payload = await response.text();
    writeFileSync(schemaPath, payload.replace(/\r\n/g, "\n"), "utf-8");
    return;
  } catch (_error) {
    const exportResult = spawnSync("python", [exportScript, schemaPath], {
      cwd: repoRoot,
      stdio: "inherit",
      env: process.env,
    });
    if (exportResult.status !== 0) {
      process.exit(exportResult.status ?? 1);
    }
  }
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

  // Fuerza un final de línea estable para evitar diffs ruidosos entre entornos.
  const generated = readFileSync(typesPath, "utf-8");
  writeFileSync(typesPath, generated.replace(/\r\n/g, "\n"), "utf-8");
})();
