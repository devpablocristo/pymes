export function cleanHeaderMenuLabel(label: string): string {
  const cleaned = label
    .trim()
    .replace(/^←\s*/u, '')
    .replace(/^Volver\s+a\s+la\s+/iu, '')
    .replace(/^Volver\s+a\s+las\s+/iu, '')
    .replace(/^Volver\s+a\s+los\s+/iu, '')
    .replace(/^Volver\s+al\s+/iu, '')
    .replace(/^Volver\s+a\s+/iu, '')
    .trim();

  if (!cleaned) return label.trim();
  return cleaned.charAt(0).toLocaleUpperCase('es-AR') + cleaned.slice(1);
}
