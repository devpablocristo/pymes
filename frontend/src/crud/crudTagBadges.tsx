/**
 * Badges de tags en tablas CRUD (misma base visual que el resto de la consola).
 */
export function renderTagBadges(tags?: string[]) {
  const list = (tags ?? []).map((t) => t.trim()).filter(Boolean);
  if (list.length === 0) {
    return <span className="text-secondary">---</span>;
  }
  return (
    <div className="crud-cell-tag-badges">
      {list.map((tag, i) => (
        <span key={`${tag}-${i}`} className="badge badge-neutral">
          {tag}
        </span>
      ))}
    </div>
  );
}
