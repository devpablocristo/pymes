import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const nginx = readFileSync(resolve(process.cwd(), "nginx.conf"), "utf8");
const dockerfile = readFileSync(
  resolve(process.cwd(), "../Dockerfile"),
  "utf8",
);
const compose = readFileSync(
  resolve(process.cwd(), "../docker-compose.yml"),
  "utf8",
);

function activeComposeHostPorts(source: string): string[] {
  const services = new Map<string, { profiled: boolean; ports: string[] }>();
  let current: { profiled: boolean; ports: string[] } | undefined;
  for (const line of source.split("\n")) {
    const service = line.match(/^  ([a-z0-9-]+):\s*$/);
    if (service?.[1]) {
      current = { profiled: false, ports: [] };
      services.set(service[1], current);
      continue;
    }
    if (current === undefined) continue;
    if (/^    profiles:/.test(line)) current.profiled = true;
    const port = line.match(
      /^    ports:\s*\["(?:(?:127\.0\.0\.1):)?([0-9]+):[0-9]+"\]\s*$/,
    );
    if (port?.[1]) current.ports.push(port[1]);
  }
  return [...services.values()]
    .filter((service) => !service.profiled)
    .flatMap((service) => service.ports);
}

function locationBody(pattern: RegExp): string {
  const match = nginx.match(pattern);
  if (!match?.[1]) {
    throw new Error("No se encontró la location esperada en nginx.conf.");
  }
  return match[1];
}

describe("Nginx productivo", () => {
  it("no publica dos servicios activos de Compose en el mismo puerto host", () => {
    const ports = activeComposeHostPorts(compose);

    expect(new Set(ports).size).toBe(ports.length);
    expect(compose).toContain('web:\n    build: { context: ., target: web-local }');
    expect(compose).toContain('ports: ["127.0.0.1:18085:8080"]');
    expect(compose).toContain('ports: ["127.0.0.1:18086:8080"]');
    expect(compose).toContain("PYMES_PREFLIGHT_TAG: local-compose-disabled");
    expect(compose).toContain(
      'PYMES_PREFLIGHT_TOKEN: "0000000000000000000000000000000000000000000000000000000000000000"',
    );
  });

  it("publica la API por el mismo origen usando sólo el upstream del runtime", () => {
    const api = locationBody(/location \/api\/\s*\{([\s\S]*?)\n  \}/);

    expect(api).toContain("proxy_pass ${PYMES_API_UPSTREAM};");
    expect(api).toContain("proxy_ssl_server_name on;");
    expect(api).toContain("proxy_hide_header X-Pymes-Release;");
    expect(api).toContain("proxy_set_header Host $proxy_host;");
    expect(dockerfile).not.toContain("ARG VITE_API_BASE_URL");
    expect(dockerfile).not.toContain("ENV VITE_API_BASE_URL");
    expect(dockerfile).toContain(
      'NGINX_ENVSUBST_FILTER="^(PYMES_API_UPSTREAM|PYMES_RELEASE_MARKER|PYMES_PREFLIGHT_TAG|PYMES_PREFLIGHT_TOKEN)$"',
    );
  });

  it("mantiene cerrada la URL pretraffic salvo para el verificador", () => {
    const api = locationBody(/location \/api\/\s*\{([\s\S]*?)\n  \}/);

    expect(nginx).toContain(
      'if ($host ~* "^${PYMES_PREFLIGHT_TAG}---") {',
    );
    expect(nginx).toContain(
      'if ($http_x_pymes_preflight_token = "${PYMES_PREFLIGHT_TOKEN}") {',
    );
    expect(nginx).toContain('if ($pymes_preflight_gate = "required") {');
    expect(api).toContain(
      'proxy_set_header X-Pymes-Preflight-Token "${PYMES_PREFLIGHT_TOKEN}";',
    );
  });

  it("expone el marcador exacto de release en API, SPA y assets", () => {
    const api = locationBody(/location \/api\/\s*\{([\s\S]*?)\n  \}/);
    const spa = locationBody(/location \/\s*\{([\s\S]*?)\n  \}/);
    const assets = locationBody(
      /location ~\* \\\.\(\?:css\|js\|map\|woff2\?\|png\|jpg\|jpeg\|gif\|svg\|ico\)\$ \{([\s\S]*?)\n  \}/,
    );

    expect(nginx).toContain(
      'add_header X-Pymes-Release "${PYMES_RELEASE_MARKER}" always;',
    );
    expect(api).not.toContain("add_header ");
    expect(spa).toContain(
      'add_header X-Pymes-Release "${PYMES_RELEASE_MARKER}" always;',
    );
    expect(assets).toContain(
      'add_header X-Pymes-Release "${PYMES_RELEASE_MARKER}" always;',
    );
  });

  it("conserva los headers de seguridad en los fallbacks SPA sin caché", () => {
    const spa = locationBody(/location \/\s*\{([\s\S]*?)\n  \}/);

    expect(spa).toContain('add_header X-Content-Type-Options "nosniff" always;');
    expect(spa).toContain('add_header X-Frame-Options "DENY" always;');
    expect(spa).toContain("add_header Content-Security-Policy");
    expect(spa).toContain('add_header Cache-Control "no-store";');
  });

  it("conserva los headers de seguridad en assets inmutables", () => {
    const assets = locationBody(
      /location ~\* \\\.\(\?:css\|js\|map\|woff2\?\|png\|jpg\|jpeg\|gif\|svg\|ico\)\$ \{([\s\S]*?)\n  \}/,
    );

    expect(assets).toContain('add_header X-Content-Type-Options "nosniff" always;');
    expect(assets).toContain('add_header X-Frame-Options "DENY" always;');
    expect(assets).toContain("add_header Content-Security-Policy");
    expect(assets).toContain('add_header Cache-Control "public, immutable";');
  });
});
