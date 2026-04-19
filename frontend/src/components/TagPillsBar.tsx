type Props = {
  tags: string[];
  value: string;
  onChange: (value: string) => void;
};

export function TagPillsBar({ tags, value, onChange }: Props) {
  if (tags.length === 0) return null;

  return (
    <div className="crud-creator-badges" role="group" aria-label="Filtrar por etiqueta">
      <button
        type="button"
        className={`badge crud-creator-badge${value === 'all' ? ' crud-creator-badge--active' : ''}`}
        aria-pressed={value === 'all'}
        onClick={() => onChange('all')}
      >
        Todos
      </button>
      {tags.map((tag) => (
        <button
          key={tag}
          type="button"
          className={`badge crud-creator-badge${value === tag ? ' crud-creator-badge--active' : ''}`}
          aria-pressed={value === tag}
          onClick={() => onChange(tag)}
        >
          {tag}
        </button>
      ))}
    </div>
  );
}
