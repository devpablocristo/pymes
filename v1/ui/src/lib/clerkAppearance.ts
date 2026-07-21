import { dark } from '@clerk/themes';

/**
 * Apariencia de Clerk alineada a la consola (tema oscuro).
 * Si más adelante sincronizamos con data-theme light/dark, se puede parametrizar acá.
 */
export const clerkAppearance = {
  baseTheme: dark,
  layout: {
    socialButtonsVariant: 'blockButton' as const,
    shimmer: false,
  },
  elements: {
    rootBox: 'clerk-root-box',
    card: 'clerk-inner-card',
  },
};
