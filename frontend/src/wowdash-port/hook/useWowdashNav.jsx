import { useMemo } from 'react';
import { useLocation } from 'react-router-dom';

/**
 * Base del laboratorio Wowdash según la URL actual (labs vs consola a pantalla completa).
 * Comentario: evita hardcodear /labs/wowdash en cada Link del template.
 */
export function useWowdashNav() {
  const { pathname } = useLocation();
  const basePath = pathname.startsWith('/console/wowdash')
    ? '/console/wowdash'
    : '/labs/wowdash';

  const w = useMemo(
    () => (suffix) => {
      if (!suffix || suffix === '/') {
        return basePath;
      }
      const s = suffix.startsWith('/') ? suffix : `/${suffix}`;
      return `${basePath}${s}`;
    },
    [basePath],
  );

  return { basePath, w };
}
