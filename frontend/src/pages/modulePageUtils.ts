import {
  resolveModuleDefault,
  type ModuleAction,
  type ModuleDefinition,
  type ModuleField,
  type ModuleRuntimeContext,
} from '../lib/moduleCatalog';
import { isReportDatasetPath } from '../lib/reportsResultPresentation';

export function currentRuntimeContext(): ModuleRuntimeContext {
  const now = new Date();
  const today = now.toISOString().slice(0, 10);
  const monthStart = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1)).toISOString().slice(0, 10);
  return { tenantId: '', today, monthStart };
}

export function buildInitialValues(fields: ModuleField[] | undefined, ctx: ModuleRuntimeContext): Record<string, string> {
  return Object.fromEntries((fields ?? []).map((field) => [field.name, resolveModuleDefault(field.defaultValue, ctx)]));
}

export function buildPath(
  template: string,
  fields: ModuleField[] | undefined,
  values: Record<string, string>,
  ctx: ModuleRuntimeContext,
): string {
  let path = template.replace(/\{\{(\w+)\}\}/g, (_match, key: string) =>
    encodeURIComponent((ctx as Record<string, string>)[key] ?? ''),
  );
  const params = new URLSearchParams();

  for (const field of fields ?? []) {
    const value = (values[field.name] ?? '').trim();
    if (field.location === 'path') {
      path = path.replace(`:${field.name}`, encodeURIComponent(value));
      continue;
    }
    if (field.location === 'query' && value !== '') {
      params.set(field.name, value);
    }
  }

  if (params.size > 0) {
    path += `${path.includes('?') ? '&' : '?'}${params.toString()}`;
  }
  return path;
}

export function appendBranchIdToReportPath(path: string, branchId: string | null | undefined): string {
  const normalizedBranchId = branchId?.trim();
  if (!normalizedBranchId || !isReportDatasetPath(path)) {
    return path;
  }
  const separator = path.includes('?') ? '&' : '?';
  return `${path}${separator}branch_id=${encodeURIComponent(normalizedBranchId)}`;
}

export function buildBody(fields: ModuleField[] | undefined, values: Record<string, string>): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  for (const field of fields ?? []) {
    if (field.location !== 'body') {
      continue;
    }
    const raw = (values[field.name] ?? '').trim();
    if (raw === '') {
      continue;
    }
    if (field.type === 'json') {
      try {
        body[field.name] = JSON.parse(raw) as unknown;
      } catch {
        throw new Error(`JSON inválido en «${field.label}»`);
      }
    } else if (field.type === 'number') {
      body[field.name] = Number(raw);
    } else {
      body[field.name] = raw;
    }
  }
  return body;
}

export function groupedModuleActions(
  module: ModuleDefinition,
): Array<{ key: string; title: string; actions: ModuleAction[] }> {
  const actions = module.actions ?? [];
  const order = module.actionGroupOrder;
  const labels = module.actionGroupLabels;
  const map = new Map<string, ModuleAction[]>();
  for (const action of actions) {
    const key = action.group ?? '_ungrouped';
    if (!map.has(key)) {
      map.set(key, []);
    }
    map.get(key)!.push(action);
  }
  const keys: string[] = [];
  if (order) {
    for (const k of order) {
      if (map.has(k)) {
        keys.push(k);
      }
    }
  }
  for (const k of map.keys()) {
    if (!keys.includes(k)) {
      keys.push(k);
    }
  }
  return keys.map((key) => ({
    key,
    title: labels?.[key] ?? (key === '_ungrouped' ? 'Acciones' : key),
    actions: map.get(key) ?? [],
  }));
}

export function missingRequiredFields(fields: ModuleField[] | undefined, values: Record<string, string>): string[] {
  return (fields ?? [])
    .filter((field) => field.required && (values[field.name] ?? '').trim() === '')
    .map((field) => field.label);
}

export function stringifyValue(value: unknown): string {
  if (value === null || value === undefined) {
    return '---';
  }
  if (typeof value === 'string') {
    return value || '---';
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value);
  }
  if (Array.isArray(value)) {
    return value.map((item) => (typeof item === 'object' ? JSON.stringify(item) : String(item))).join(', ');
  }
  return JSON.stringify(value);
}

function isScalarCell(value: unknown): boolean {
  if (value === null || value === undefined) {
    return true;
  }
  const t = typeof value;
  return t === 'string' || t === 'number' || t === 'boolean';
}

export function tableColumnsForRows(rows: Array<Record<string, unknown>>, maxCols: number): string[] {
  const allKeys = Array.from(new Set(rows.flatMap((row) => Object.keys(row))));
  const scalarKeys = allKeys.filter((key) => rows.every((row) => isScalarCell(row[key])));
  const ordered = scalarKeys.length > 0 ? scalarKeys : allKeys;
  const priority = (k: string): number => {
    if (k === 'id') {
      return 0;
    }
    if (k.endsWith('_id') || k.endsWith('Id')) {
      return 1;
    }
    return 2;
  };
  ordered.sort((a, b) => {
    const pa = priority(a);
    const pb = priority(b);
    if (pa !== pb) {
      return pa - pb;
    }
    return a.localeCompare(b);
  });
  return ordered.slice(0, maxCols);
}

export function extractRows(data: unknown): Array<Record<string, unknown>> | null {
  if (Array.isArray(data) && data.every((item) => item && typeof item === 'object')) {
    return data as Array<Record<string, unknown>>;
  }
  if (data && typeof data === 'object' && 'items' in data) {
    const raw = (data as { items: unknown }).items;
    if (raw == null) {
      return [];
    }
    if (!Array.isArray(raw)) {
      return null;
    }
    if (raw.length === 0) {
      return [];
    }
    if (raw.every((item) => item && typeof item === 'object')) {
      return raw as Array<Record<string, unknown>>;
    }
  }
  if (data && typeof data === 'object' && 'data' in data) {
    const inner = (data as { data: unknown }).data;
    if (Array.isArray(inner)) {
      if (inner.length === 0) {
        return [];
      }
      if (inner.every((item) => item && typeof item === 'object')) {
        return inner as Array<Record<string, unknown>>;
      }
      return null;
    }
    if (inner && typeof inner === 'object' && !Array.isArray(inner)) {
      const row = inner as Record<string, unknown>;
      const scalarRow = Object.fromEntries(Object.entries(row).filter(([, v]) => isScalarCell(v)));
      if (Object.keys(scalarRow).length > 0) {
        return [scalarRow];
      }
    }
  }
  return null;
}
