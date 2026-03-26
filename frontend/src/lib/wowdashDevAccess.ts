/**
 * Acceso opcional al laboratorio Wowdash en el host de desarrollo.
 * Comentario: con Clerk + onboarding incompleto, /console/wowdash redirige y parece “roto”.
 * Con `VITE_DEV_WOWDASH_OPEN=true` solo en `vite` (import.meta.env.DEV), la ruta abre sin auth.
 * En `vite build` / preview, import.meta.env.DEV es false → nunca se abre en prod por este flag.
 */
export function wowdashOpenInDev(): boolean {
  return Boolean(import.meta.env.DEV && import.meta.env.VITE_DEV_WOWDASH_OPEN === 'true');
}
