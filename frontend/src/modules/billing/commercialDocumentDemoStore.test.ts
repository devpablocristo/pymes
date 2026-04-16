import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  archiveCommercialDocument,
  readCommercialDocumentDemoRecords,
  restoreCommercialDocument,
  writeCommercialDocumentDemoRecords,
} from './commercialDocumentDemoStore';

describe('commercialDocumentDemoStore', () => {
  beforeEach(() => {
    window.localStorage.clear();
    vi.useRealTimers();
  });

  it('archives and restores generic commercial documents', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-04-11T12:00:00.000Z'));
    const doc = { id: '1', archived_at: null };

    const archived = archiveCommercialDocument(doc);
    expect(archived.archived_at).toBe('2026-04-11T12:00:00.000Z');
    expect(restoreCommercialDocument(archived).archived_at).toBeNull();
  });

  it('persists and reads demo records through localStorage', () => {
    const records = [{ id: '1', number: 'INV-1' }];
    writeCommercialDocumentDemoRecords('demo-key', records);

    expect(
      readCommercialDocumentDemoRecords('demo-key', [], (raw) =>
        raw && typeof raw === 'object' ? ({ ...(raw as Record<string, unknown>) } as { id: string; number: string }) : null,
      ),
    ).toEqual(records);
  });
});
