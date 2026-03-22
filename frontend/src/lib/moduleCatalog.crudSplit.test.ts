import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

describe('moduleCatalog CRUD split', () => {
  it('does not statically import the heavyweight resourceConfigs module', () => {
    const dir = path.dirname(fileURLToPath(import.meta.url));
    const src = readFileSync(path.join(dir, 'moduleCatalog.ts'), 'utf8');

    expect(src).toContain("../crud/crudModuleCatalog");
    expect(src).not.toContain("../crud/resourceConfigs");
  });
});
