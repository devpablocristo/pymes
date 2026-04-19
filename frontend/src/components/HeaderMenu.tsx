import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { tenantLink, useTenantSlug } from '../lib/tenantSlug';
import { type HeaderMenuItem } from './HeaderMenuContext';
import './SettingsMenu.css';

export function HeaderMenu({ items = [] }: { items?: HeaderMenuItem[] }) {
  const slug = useTenantSlug();
  const navigate = useNavigate();
  const rootRef = useRef<HTMLDivElement | null>(null);
  const buttonRef = useRef<HTMLButtonElement | null>(null);
  const [open, setOpen] = useState(false);
  const resolvedItems: HeaderMenuItem[] = [
    ...items,
    { label: 'Ajustes', href: tenantLink('/settings', slug) },
  ];

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      if (!open) return;
      setOpen(false);
      buttonRef.current?.focus();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [open]);

  useEffect(() => {
    const handlePointerDown = (event: MouseEvent) => {
      if (!open) return;
      if (rootRef.current?.contains(event.target as Node)) return;
      setOpen(false);
    };
    window.addEventListener('mousedown', handlePointerDown);
    return () => window.removeEventListener('mousedown', handlePointerDown);
  }, [open]);

  return (
    <div ref={rootRef} className="settings-menu">
      <button
        ref={buttonRef}
        type="button"
        className="settings-menu__trigger"
        aria-label="Abrir menú"
        aria-expanded={open}
        title="Menú"
        onClick={() => setOpen((current) => !current)}
      >
        <span className="settings-menu__avatar" aria-hidden>
          <span className="settings-menu__avatar-head" />
          <span className="settings-menu__avatar-body" />
        </span>
      </button>
      {open ? (
        <div className="settings-menu__panel">
          {resolvedItems.map((item) => (
            <button
              key={`${item.label}:${item.href}`}
              type="button"
              className="settings-menu__item"
              onClick={() => {
                setOpen(false);
                item.onSelect?.();
                navigate(item.href);
              }}
            >
              {item.label}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}
