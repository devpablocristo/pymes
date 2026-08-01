import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const nginx = readFileSync(resolve(process.cwd(), "nginx.conf"), "utf8");

function locationBody(pattern: RegExp): string {
  const match = nginx.match(pattern);
  if (!match?.[1]) {
    throw new Error("No se encontró la location esperada en nginx.conf.");
  }
  return match[1];
}

describe("Nginx productivo", () => {
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
