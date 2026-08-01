import { existsSync, mkdtempSync, rmSync } from "node:fs";
import { spawn, spawnSync } from "node:child_process";
import { tmpdir } from "node:os";
import { join } from "node:path";

const candidates = [process.env.PLAYWRIGHT_CHROME_PATH, "/usr/bin/google-chrome", "/usr/bin/chromium"].filter(Boolean);
let executable = candidates.find((candidate) => existsSync(candidate));
const cli = new URL("../node_modules/@playwright/test/cli.js", import.meta.url);
const vite = new URL("../node_modules/vite/bin/vite.js", import.meta.url);
if (!executable) {
  const install = spawnSync(process.execPath, [cli.pathname, "install", "chromium"], {
    stdio: "inherit",
    env: process.env,
  });
  if (install.status !== 0) {
    process.exit(install.status ?? 1);
  }
}
const outputDir = mkdtempSync(join(tmpdir(), "pymes-web-e2e-"));
const build = spawnSync(
  process.execPath,
  [
    vite.pathname,
    "build",
    "--mode",
    "e2e",
    "--outDir",
    outputDir,
    "--emptyOutDir",
  ],
  {
    stdio: "inherit",
    env: process.env,
  },
);
if (build.status !== 0) {
  rmSync(outputDir, { recursive: true, force: true });
  process.exit(build.status ?? 1);
}

const server = spawn(
  process.execPath,
  [
    vite.pathname,
    "preview",
    "--host",
    "127.0.0.1",
    "--port",
    "4173",
    "--outDir",
    outputDir,
  ],
  {
    stdio: ["ignore", "inherit", "inherit"],
    env: process.env,
  },
);

function cleanup() {
  if (server.exitCode === null) {
    server.kill("SIGTERM");
  }
  rmSync(outputDir, { recursive: true, force: true });
}

async function waitForServer() {
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    if (server.exitCode !== null) {
      throw new Error(`Vite terminó antes de iniciar (${server.exitCode}).`);
    }
    try {
      const response = await fetch("http://127.0.0.1:4173/healthz");
      if (response.ok) return;
    } catch {
      // Vite todavía está iniciando.
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error("Vite no respondió dentro de 30 segundos.");
}

try {
  await waitForServer();
} catch (error) {
  cleanup();
  throw error;
}

const child = spawn(process.execPath, [cli.pathname, "test", ...process.argv.slice(2)], {
  stdio: "inherit",
  env: {
    ...process.env,
    PLAYWRIGHT_EXTERNAL_SERVER: "true",
    ...(executable ? { PLAYWRIGHT_CHROME_PATH: executable } : {}),
  },
});

child.on("exit", (code, signal) => {
  cleanup();
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exitCode = code ?? 1;
});

for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, () => {
    child.kill(signal);
    cleanup();
  });
}
