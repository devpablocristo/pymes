/** Evita `<img src>` con texto arbitrario (p. ej. notas en el campo URLs). */
export function isDisplayableCrudImageSrc(raw: string): boolean {
  const s = raw.trim();
  if (!s) return false;
  if (s.startsWith('data:image/')) return true;
  if (s.startsWith('blob:')) return true;
  if (s.startsWith('/')) return true;
  try {
    const u = new URL(s);
    return u.protocol === 'http:' || u.protocol === 'https:';
  } catch {
    return false;
  }
}

/**
 * Lista de imágenes almacenada como texto (una entrada por línea).
 * Importante: no partir por comas — los data URLs llevan `data:image/png;base64,...` y se romperían.
 */
export function parseCrudLinkedEntityImageUrlList(value: string | undefined | null): string[] {
  const normalized = String(value ?? '').trim();
  if (!normalized) return [];
  const parts = normalized.split(/\r?\n/).map((part) => part.trim()).filter(Boolean);
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

/** URLs desde `image_urls` top-level, `metadata.image_urls` o legados `image_url` / `imageUrl`. */
export function extractCrudRecordImageUrls(record: Record<string, unknown>): string[] {
  const top =
    Array.isArray(record.image_urls) ?
      record.image_urls
        .filter((v): v is string => typeof v === 'string' && v.trim().length > 0)
        .map((s) => s.trim())
    : [];

  let fromMeta: string[] = [];
  const meta = record.metadata;
  if (meta && typeof meta === 'object' && !Array.isArray(meta)) {
    const raw = (meta as Record<string, unknown>).image_urls;
    if (Array.isArray(raw)) {
      fromMeta = raw
        .filter((v): v is string => typeof v === 'string' && v.trim().length > 0)
        .map((s) => s.trim());
    }
  }

  const legacySingle =
    typeof record.image_url === 'string' && record.image_url.trim().length > 0
      ? record.image_url.trim()
      : typeof record.imageUrl === 'string' && record.imageUrl.trim().length > 0
        ? record.imageUrl.trim()
        : undefined;

  return collectCrudImageUrls({
    imageUrls: [...top, ...fromMeta],
    legacyImageUrl: legacySingle,
  });
}

export function formatCrudRecordImageUrlsToForm(record: Record<string, unknown>): string {
  return extractCrudRecordImageUrls(record).join('\n');
}

/** Miniatura en galerías: última URL que el `<img>` puede mostrar (orden lista = última subida al hacer append). */
export function pickGalleryHeroCrudImageSrc(record: Record<string, unknown>): string | undefined {
  const urls = extractCrudRecordImageUrls(record);
  for (let i = urls.length - 1; i >= 0; i--) {
    const u = urls[i];
    if (isDisplayableCrudImageSrc(u)) return u.trim();
  }
  return undefined;
}

export function formatCrudLinkedEntityImageUrlsToForm(
  urls: string[] | undefined,
  legacySingle?: string,
): string {
  const list = urls?.length ? urls : legacySingle?.trim() ? [legacySingle.trim()] : [];
  return list.join('\n');
}
