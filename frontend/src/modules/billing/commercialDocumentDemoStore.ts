export type CommercialDocumentArchivedRecord = {
  archived_at?: string | null;
};

export function archiveCommercialDocument<T extends CommercialDocumentArchivedRecord>(document: T): T {
  return { ...document, archived_at: new Date().toISOString() };
}

export function restoreCommercialDocument<T extends CommercialDocumentArchivedRecord>(document: T): T {
  return { ...document, archived_at: null };
}

export function readCommercialDocumentDemoRecords<T>(
  storageKey: string,
  initialRecords: T[],
  sanitizeRecord: (raw: unknown) => T | null,
): T[] {
  if (typeof window === 'undefined') return initialRecords;
  try {
    const raw = window.localStorage.getItem(storageKey);
    if (!raw) return initialRecords;
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return initialRecords;
    const records = parsed.map(sanitizeRecord).filter(Boolean) as T[];
    return records.length ? records : initialRecords;
  } catch {
    return initialRecords;
  }
}

export function writeCommercialDocumentDemoRecords<T>(storageKey: string, records: T[]): void {
  if (typeof window === 'undefined') return;
  window.localStorage.setItem(storageKey, JSON.stringify(records));
}
