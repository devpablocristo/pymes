export function parseCrudLinkedEntityImageUrlList(value: string | undefined | null): string[] {
  const normalized = String(value ?? '').trim();
  if (!normalized) return [];
  const parts = normalized
    .split(/[\n,]+/)
    .map((part) => part.trim())
    .filter(Boolean);
  const out: string[] = [];
  const seen = new Set<string>();
  for (const part of parts) {
    if (seen.has(part)) continue;
    seen.add(part);
    out.push(part);
  }
  return out;
}

export function collectCrudImageUrls(input: {
  imageUrls?: string[] | null;
  legacyImageUrl?: string | null;
}): string[] {
  const raw = input.imageUrls?.length ? input.imageUrls : input.legacyImageUrl?.trim() ? [input.legacyImageUrl.trim()] : [];
  const out: string[] = [];
  const seen = new Set<string>();
  for (const value of raw) {
    const trimmed = (value ?? '').trim();
    if (!trimmed || seen.has(trimmed)) continue;
    seen.add(trimmed);
    out.push(trimmed);
  }
  return out;
}

export function formatCrudLinkedEntityImageUrlsToForm(
  urls: string[] | undefined,
  legacySingle?: string,
): string {
  const list = urls?.length ? urls : legacySingle?.trim() ? [legacySingle.trim()] : [];
  return list.join('\n');
}
