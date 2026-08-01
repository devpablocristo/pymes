import { readdir, readFile } from "node:fs/promises";
import { extname, join } from "node:path";

const dist = new URL("../dist/", import.meta.url);
const assets = new URL("assets/", dist);
const files = await readdir(assets);

const sourceMaps = files.filter((file) => file.endsWith(".map"));
if (sourceMaps.length > 0) {
  throw new Error(
    `El bundle productivo publica source maps: ${sourceMaps.join(", ")}.`,
  );
}

const testChunks = files.filter((file) =>
  /(?:fake|local-session|localsession)/i.test(file),
);
if (testChunks.length > 0) {
  throw new Error(
    `El bundle productivo contiene chunks exclusivos de test: ${testChunks.join(", ")}.`,
  );
}

const fakeMarkers = [
  "org_e2e",
  "Reserva simultánea",
  "Capacitación interna",
  "Cliente E2E",
  "local-e2e-token",
];
for (const file of files.filter((entry) => extname(entry) === ".js")) {
  const contents = await readFile(join(assets.pathname, file), "utf8");
  for (const marker of fakeMarkers) {
    if (contents.includes(marker)) {
      throw new Error(
        `El bundle productivo ${file} contiene datos del fake: ${marker}.`,
      );
    }
  }
}

console.log("Bundle productivo sin fakes ni source maps públicos.");
