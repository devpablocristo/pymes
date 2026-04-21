function normalizeCrudImageUrlParts(parts: string[]): string[] {
  const out: string[] = [];
  const seen = new Set<string>();
  let lastDataPrefix = '';
  for (let index = 0; index < parts.length; index += 1) {
    let current = String(parts[index] ?? '').trim();
    if (!current) continue;
    if (current.startsWith('data:image/') && !current.includes(',')) {
      const next = String(parts[index + 1] ?? '').trim();
      if (next) {
        current = `${current},${next}`;
        index += 1;
      }
    }
    if (current.startsWith('data:image/')) {
      const commaIndex = current.indexOf(',');
      if (commaIndex > 0) {
        lastDataPrefix = current.slice(0, commaIndex + 1);
      }
    } else if (looksLikeCrudImageBase64(current)) {
      const prefix = lastDataPrefix || inferCrudImageDataPrefix(current);
      if (prefix) current = `${prefix}${current}`;
    }
    if (seen.has(current)) continue;
    seen.add(current);
    out.push(current);
  }
  return out;
}

function inferCrudImageDataPrefix(raw: string): string {
  const trimmed = String(raw ?? '').trim();
  if (trimmed.startsWith('/9j/')) return 'data:image/jpeg;base64,';
  if (trimmed.startsWith('iVBOR')) return 'data:image/png;base64,';
  if (trimmed.startsWith('R0lGOD')) return 'data:image/gif;base64,';
  if (trimmed.startsWith('UklGR')) return 'data:image/webp;base64,';
  return '';
}

function looksLikeCrudImageBase64(raw: string): boolean {
  const trimmed = String(raw ?? '').trim();
  if (trimmed.length < 8 || inferCrudImageDataPrefix(trimmed) === '') return false;
  return /^[A-Za-z0-9+/=]+$/.test(trimmed);
}

export function parseCrudLinkedEntityImageUrlList(value: string | undefined | null): string[] {
  const normalized = String(value ?? '').trim();
  if (!normalized) return [];
  return normalizeCrudImageUrlParts(normalized.split(/\n+/));
}

export function collectCrudImageUrls(input: {
  imageUrls?: string[] | null;
  legacyImageUrl?: string | null;
}): string[] {
  const raw = input.imageUrls?.length ? input.imageUrls : input.legacyImageUrl?.trim() ? [input.legacyImageUrl.trim()] : [];
  return normalizeCrudImageUrlParts(raw);
}

export function formatCrudLinkedEntityImageUrlsToForm(
  urls: string[] | undefined,
  legacySingle?: string,
): string {
  const list = urls?.length ? urls : legacySingle?.trim() ? [legacySingle.trim()] : [];
  return normalizeCrudImageUrlParts(list).join('\n');
}
