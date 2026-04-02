/**
 * Paleta de producto: **hex** estable para API / FullCalendar; **token** para UI (tema claro/oscuro).
 * Una sola lista — evita divergencia entre `CalendarPage` y `UIComponentsPage`.
 */
export type ProductPaletteEntry = {
  id: string;
  label: string;
  hex: string;
  token: string;
};

export const PRODUCT_PALETTE: readonly ProductPaletteEntry[] = [
  { id: 'primary', label: 'Primary', hex: '#3b82f6', token: 'var(--color-primary)' },
  { id: 'success', label: 'Success', hex: '#10b981', token: 'var(--color-success)' },
  { id: 'warning', label: 'Warning', hex: '#f59e0b', token: 'var(--color-warning)' },
  { id: 'danger', label: 'Danger', hex: '#ef4444', token: 'var(--color-danger)' },
  { id: 'purple', label: 'Purple', hex: '#8b5cf6', token: 'var(--color-purple)' },
  { id: 'pink', label: 'Pink', hex: '#ec4899', token: 'var(--color-accent-pink)' },
  { id: 'cyan', label: 'Cyan', hex: '#06b6d4', token: 'var(--color-accent-cyan)' },
  { id: 'neutral', label: 'Neutral', hex: '#64748b', token: 'var(--color-text-secondary)' },
] as const;

/** Etiquetas ES para ajustes de marca (Theme hub); ids alineados a `PRODUCT_PALETTE`. */
export const PRODUCT_PALETTE_LABELS_ES: Record<string, string> = {
  primary: 'Azul',
  success: 'Verde',
  warning: 'Naranja',
  danger: 'Rojo',
  purple: 'Violeta',
  pink: 'Rosa',
  cyan: 'Cian',
  neutral: 'Neutro',
};

/** Swatches del hub de tema (6 primeros tonos); `bg` = token CSS. */
export function themeHubColorSwatches(): { id: string; label: string; bg: string }[] {
  return PRODUCT_PALETTE.slice(0, 6).map((p) => ({
    id: p.id,
    label: PRODUCT_PALETTE_LABELS_ES[p.id] ?? p.label,
    bg: p.token,
  }));
}

/** Default para citas nuevas y respuestas API sin color. */
export const DEFAULT_APPOINTMENT_COLOR_HEX = PRODUCT_PALETTE[0].hex;

/** Selector de color del calendario: hex persiste; swatch usa token. */
export const CALENDAR_APPOINTMENT_COLOR_OPTIONS: { hex: string; swatch: string }[] =
  PRODUCT_PALETTE.map((p) => ({ hex: p.hex, swatch: p.token }));
