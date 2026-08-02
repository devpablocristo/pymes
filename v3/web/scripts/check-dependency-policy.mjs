import { readFile } from "node:fs/promises";

const packageJson = JSON.parse(
  await readFile(new URL("../package.json", import.meta.url), "utf8"),
);
const packageLock = JSON.parse(
  await readFile(new URL("../package-lock.json", import.meta.url), "utf8"),
);

const required = new Map([
  ["@devpablocristo/platform-calendar-board", "0.2.0"],
  ["@devpablocristo/platform-scheduling", "0.2.0"],
  ["@fullcalendar/core", "6.1.21"],
  ["@fullcalendar/daygrid", "6.1.21"],
  ["@fullcalendar/interaction", "6.1.21"],
  ["@fullcalendar/list", "6.1.21"],
  ["@fullcalendar/luxon3", "6.1.21"],
  ["@fullcalendar/react", "6.1.21"],
  ["@fullcalendar/timegrid", "6.1.21"],
  ["luxon", "3.7.2"],
  ["react", "19.2.6"],
  ["react-dom", "19.2.6"],
]);

const dependencies = {
  ...packageJson.dependencies,
  ...packageJson.devDependencies,
};

for (const [name, version] of required) {
  if (dependencies[name] !== version) {
    throw new Error(`${name} debe permanecer fijado exactamente en ${version}.`);
  }
}

const bannedNames = [
  "@fullcalendar/premium-common",
  "@fullcalendar/resource",
  "@fullcalendar/resource-daygrid",
  "@fullcalendar/resource-timegrid",
  "@fullcalendar/resource-timeline",
  "@fullcalendar/rrule",
  "rrule",
];
for (const name of bannedNames) {
  if (dependencies[name] !== undefined || packageLock.packages?.[`node_modules/${name}`]) {
    throw new Error(`Dependencia fuera del alcance del MVP: ${name}.`);
  }
}

const localProtocol = /^(?:file|link|workspace):/;
for (const [name, version] of Object.entries(dependencies)) {
  if (localProtocol.test(String(version))) {
    throw new Error(`${name} usa una ruta local prohibida: ${version}.`);
  }
}
for (const [path, metadata] of Object.entries(packageLock.packages ?? {})) {
  const resolved = String(metadata?.resolved ?? "");
  if (localProtocol.test(resolved)) {
    throw new Error(`${path || "root"} se resuelve mediante una ruta local: ${resolved}.`);
  }
}

console.log("Política de dependencias Web verificada.");
