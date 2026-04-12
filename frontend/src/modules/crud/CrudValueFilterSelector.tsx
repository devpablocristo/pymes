import type { CrudValueFilterOption } from '../../components/CrudPage';

type Props<T extends { id: string }> = {
  value: string;
  onChange: (value: string) => void;
  options: CrudValueFilterOption<T>[];
  allLabel?: string;
  ariaLabel?: string;
  className?: string;
};

export function CrudValueFilterSelector<T extends { id: string }>({
  value,
  onChange,
  options,
  allLabel = 'Todas',
  ariaLabel = 'Filtrar por valor',
  className,
}: Props<T>) {
  if (options.length === 0) return null;
  return (
    <select
      className={className}
      aria-label={ariaLabel}
      value={value}
      onChange={(event) => onChange(event.target.value)}
    >
      <option value="all">{allLabel}</option>
      {options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  );
}
