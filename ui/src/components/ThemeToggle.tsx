import { useEffect, useState } from 'react';
import { IconMoon, IconSun } from '@tabler/icons-react';
import { useI18n } from '../lib/i18n';
import { getTheme, toggleTheme } from '../lib/theme';

/**
 * ThemeToggle — botón compacto para alternar entre light y dark.
 *
 * Usa el theme manager existente (`pymes-ui`/`pymes:theme`). El click
 * llama a `toggleTheme` y actualiza el estado local para refrescar el
 * ícono y el aria-label sin esperar al próximo re-render del shell.
 *
 * Pensado para montarse en la topbar; se puede usar también en cualquier
 * otra superficie (drawer, settings, etc.) sin cambios.
 */
export function ThemeToggle({ className }: { className?: string }) {
  const { t } = useI18n();
  const [theme, setTheme] = useState<string>(() => getTheme());

  // Sincronizar si otra pestaña cambia el tema (storage event).
  useEffect(() => {
    const sync = () => setTheme(getTheme());
    window.addEventListener('storage', sync);
    return () => window.removeEventListener('storage', sync);
  }, []);

  const isDark = theme === 'dark';
  const handleClick = () => {
    toggleTheme();
    setTheme(getTheme());
  };

  return (
    <button
      type="button"
      onClick={handleClick}
      className={className ?? 'theme-toggle'}
      aria-label={isDark ? t('shell.theme.light') : t('shell.theme.dark')}
      title={isDark ? t('shell.theme.light') : t('shell.theme.dark')}
    >
      {isDark ? <IconSun size={18} stroke={1.6} /> : <IconMoon size={18} stroke={1.6} />}
    </button>
  );
}
