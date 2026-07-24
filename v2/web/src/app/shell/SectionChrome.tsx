import type { ReactNode } from "react";
import { AccountMenu } from "./AccountMenu";

type SectionHeaderProps = {
  title: ReactNode;
  subtitle?: ReactNode;
};

type SectionSearchProps = {
  label: string;
  placeholder: string;
  value: string;
  onChange: (value: string) => void;
};

export function SectionHeader({ title, subtitle }: SectionHeaderProps) {
  return (
    <header className="page-topbar">
      <div className="page-topbar__title">
        <h1>{title}</h1>
        {subtitle ? <small>{subtitle}</small> : null}
      </div>
      <AccountMenu />
    </header>
  );
}

export function SectionSearch({
  label,
  placeholder,
  value,
  onChange,
}: SectionSearchProps) {
  return (
    <label className="section-search">
      <span className="visually-hidden">{label}</span>
      <svg aria-hidden="true" viewBox="0 0 24 24" fill="none">
        <circle cx="11" cy="11" r="6.5" />
        <path d="m16 16 4 4" />
      </svg>
      <input
        aria-label={label}
        autoComplete="off"
        placeholder={placeholder}
        type="search"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </label>
  );
}
