import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

describe('moduleCatalog CRUD split', () => {
  it('does not statically import the heavyweight resourceConfigs module', () => {
    const dir = path.dirname(fileURLToPath(import.meta.url));
    const src = readFileSync(path.join(dir, 'moduleCatalog.ts'), 'utf8');

    expect(src).toContain('../crud/crudModuleCatalog');
    expect(src).not.toContain('../crud/resourceConfigs');
  });

  it('keeps work order screens on the lazy CRUD loader', () => {
    const dir = path.dirname(fileURLToPath(import.meta.url));
    const workshopsCrudSrc = readFileSync(path.join(dir, '../crud/resourceConfigs.workshops.tsx'), 'utf8');
    const workOrdersFactorySrc = readFileSync(path.join(dir, '../modules/work-orders/workOrdersCrudFactory.tsx'), 'utf8');
    const shellRoutesSrc = readFileSync(path.join(dir, '../app/ShellRoutes.tsx'), 'utf8');
    const configuredCrudSrc = readFileSync(path.join(dir, '../crud/configuredCrudViews.tsx'), 'utf8');

    const usesLazyCrudBoundary = (src: string) =>
      src.includes('PymesSimpleCrudListModeContent') || src.includes('ConfiguredCrudModePage');

    expect(workshopsCrudSrc).not.toContain('../pages/modes/CarWorkOrders');
    expect(usesLazyCrudBoundary(workOrdersFactorySrc)).toBe(true);
    expect(usesLazyCrudBoundary(shellRoutesSrc)).toBe(true);
    expect(shellRoutesSrc).not.toContain('WorkOrdersKanbanPanel');
    expect(configuredCrudSrc).toContain('./lazyCrudPage');
  });

  it('splits operations and control CRUD groups into dedicated lazy modules', () => {
    const dir = path.dirname(fileURLToPath(import.meta.url));
    const lazyCrudSrc = readFileSync(path.join(dir, '../crud/lazyCrudPage.tsx'), 'utf8');

    expect(lazyCrudSrc).toContain("'cashflow'");
    expect(lazyCrudSrc).toContain("import('./resourceConfigs.operations')");
    expect(lazyCrudSrc).toContain("'procurementRequests'");
    expect(lazyCrudSrc).toContain("import('./resourceConfigs.governance')");
    expect(lazyCrudSrc).toContain("'attachments'");
    expect(lazyCrudSrc).toContain("'audit'");
    expect(lazyCrudSrc).toContain("import('./resourceConfigs.control')");
  });

  it('keeps shell layout free from clerk auth widgets', () => {
    const dir = path.dirname(fileURLToPath(import.meta.url));
    const shellSrc = readFileSync(path.join(dir, '../shared/frontendShell.tsx'), 'utf8');
    const authSrc = readFileSync(path.join(dir, '../shared/frontendAuth.tsx'), 'utf8');

    expect(shellSrc).not.toContain('@clerk/react');
    expect(authSrc).toContain('@clerk/react');
  });
});
