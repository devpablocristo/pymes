import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const root = resolve(new URL("..", import.meta.url).pathname);
const expected = join(root, "src/api/generated.ts");
const contract = resolve(root, "../api/openapi.yaml");
const temporary = mkdtempSync(join(tmpdir(), "pymes-web-openapi-"));
const generated = join(temporary, "generated.ts");
const cli = join(root, "node_modules/openapi-typescript/bin/cli.js");

try {
  const result = spawnSync(process.execPath, [cli, contract, "--output", generated], {
    cwd: root,
    stdio: "inherit",
  });
  if (result.status !== 0) {
    process.exitCode = result.status ?? 1;
  } else if (readFileSync(expected, "utf8") !== readFileSync(generated, "utf8")) {
    console.error("src/api/generated.ts no coincide con ../api/openapi.yaml; ejecutá npm run generate:api.");
    process.exitCode = 1;
  }
} finally {
  rmSync(temporary, { recursive: true, force: true });
}
