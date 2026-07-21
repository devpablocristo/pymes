export function renderTagBadges(tags?: string[]) {
  const list = (tags ?? []).map((t) => t.trim()).filter(Boolean);
  if (list.length === 0) {
    return '';
  }
  return list.join(', ');
}
